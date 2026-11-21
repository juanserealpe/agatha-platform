"""AGATHA · Implementación de referencia de MotionAssess motion-v2 (Rule Engine, MOVIMIENTO).
motion-v2 = motion-v1 (pasos 1-6 iguales) + tabla de lo normal por dispositivo (reglas cortas y largas),
nivel calculado / nivel enviado, historial al abrir, resumen diario y cierre con cambio total.
Uso: python3 motion_v2_ref.py [input.csv] [salida.csv]   (por defecto motion_input.csv -> motion_v2_expected.csv)
Hora t = fin de la ventana: la hora t cubre [t-1h, t), como feature_evaluations."""
import pandas as pd, numpy as np, json

P = dict(  # motion_threshold_versions v1 del MP-11556
    records_per_hour=4, min_records_hour=3,
    jump_deg=0.03, deriva_jump_deg=0.03,
    shift_deg=0.04, shift_window_hours=2, shift_min_records=6,
    base_from_hours=24, base_to_hours=6, base_min_records=36,
    gap_min_hours=1.0, gap_pre_hours=2, gap_post_minutes=45, gap_pre_min_records=3, gap_post_min_records=2,
    return_fraction=0.5, shift_suppress_after_return_hours=24,
    abrupt_suppress_after_joint_hours=6, slow_drift_deg_24h=0.04,   # abrupt_deg/abrupt_hours: reemplazados por rules
    still_hours=24, still_min_calculated=20, still_range_deg=0.02,
    temp_flat_rate=0.03, temp_flat_hours=6, temp_flat_min_rates=5,
    temp_flow_rate=0.08, temp_flow_hours=2,
    stabilize_max_hours=36,
    still_stable_first_hours=6, still_stable_every_hours=12,
    trend_deg_7d=0.04,
    temp_mode='MOVIMIENTO_Y_TEMPERATURA',
    # ---- motion-v2: tabla de lo normal (amarillo_deg = desde aquí es AMARILLO; lo normal es menos)
    rules=[dict(window_hours=1,   axis='ANY', amarillo_deg=0.04, naranja_deg=None, rojo_deg=0.20),
           dict(window_hours=3,   axis='ANY', amarillo_deg=0.04, naranja_deg=0.10, rojo_deg=None),
           dict(window_hours=72,  axis='ANY', amarillo_deg=0.08, naranja_deg=None, rojo_deg=None),
           dict(window_hours=168, axis='ANY', amarillo_deg=0.10, naranja_deg=0.20, rojo_deg=None),
           dict(window_hours=720, axis='ANY', amarillo_deg=0.15, naranja_deg=0.25, rojo_deg=None)],
    send_naranja='OBSERVATION', send_rojo='OBSERVATION',
    long_hold_hours=12, long_min_coverage=0.5, long_close_hours=72, long_close_net_deg=0.02,
    closure_cap_level='AMARILLO', closure_max_deg=0.20,
    daily_summary=True,
    history_step_deg=0.015, history_step_share=0.7, history_step_window_hours=6, history_step_group_hours=8,
    history_days=(7, 30), history_previous_days=90)

H = pd.Timedelta('1h')
ORD = {'NORMAL': 0, 'AMARILLO': 1, 'NARANJA': 2, 'ROJO': 3}
LEVELS = ['NORMAL', 'AMARILLO', 'NARANJA', 'ROJO']
EPS = 1e-9


def thresholds(p):
    """filas de la tabla -> lista (clave, window_hours, kind, axis, grados, nivel); kind SHORT si ventana <= 24 h"""
    out = []
    for r in p['rules']:
        kind = 'SHORT' if r['window_hours'] <= 24 else 'LONG'
        for lev, key in (('AMARILLO', 'amarillo_deg'), ('NARANJA', 'naranja_deg'), ('ROJO', 'rojo_deg')):
            if r.get(key) is not None:
                out.append((f"{r['window_hours']}:{r['axis']}:{lev}", r['window_hours'], kind, r['axis'], r[key], lev))
    return out


def sent_level(calc, p):
    """nivel que se envía: el más alto ACTIVO que no supera el calculado"""
    active = ['AMARILLO'] + (['NARANJA'] if p['send_naranja'] == 'ACTIVE' else []) + (['ROJO'] if p['send_rojo'] == 'ACTIVE' else [])
    best = 'NORMAL'
    for l in active:
        if ORD[l] <= ORD[calc] and ORD[l] > ORD[best]: best = l
    return best


def axes(a):
    return [('pitch', 0), ('roll', 1)] if a == 'ANY' else [(a.lower(), 0 if a == 'PITCH' else 1)]


import bisect


class Levels:
    """niveles horarios de las horas calculadas, en orden (para buscar por rango)"""
    def __init__(self): self.hrs = []; self.v = {}
    def add(self, h, x): self.hrs.append(h); self.v[h] = x
    def get(self, h): return self.v.get(h)
    def between(self, a, b):  # horas calculadas con a <= h <= b
        i = bisect.bisect_left(self.hrs, a); j = bisect.bisect_right(self.hrs, b)
        return self.hrs[i:j]


def lmed(L, a, b, i):
    v = [L.v[h][i] for h in L.between(a, b)]
    return float(np.median(v)) if v else None


def last_level(L, tb, i, n=4):
    j = bisect.bisect_right(L.hrs, tb)
    v = [L.v[h][i] for h in L.hrs[max(0, j - n):j]]
    return float(np.median(v)) if v else None


def steps(L, a, b, i, p):
    """escalones del eje i en (a, b]: horas con |nivel(h) - nivel(h - 6 h)| >= 0,015, agrupadas si están a <= 8 h"""
    W = p['history_step_window_hours'] * H
    groups = []
    for h in L.between(a + pd.Timedelta(seconds=1), b):
        prv = L.get(h - W)
        if not prv: continue
        d = L.v[h][i] - prv[i]
        if abs(d) >= p['history_step_deg'] - EPS:
            if groups and h - groups[-1]['last'] <= p['history_step_group_hours'] * H:
                g = groups[-1]; g['last'] = h
                if abs(d) > abs(g['delta']): g['delta'] = d
            else:
                groups.append(dict(at=h, last=h, delta=d))
    return [dict(at=g['at'], delta=round(g['delta'], 3)) for g in groups]


def step_pattern(st, net, p):
    """ESCALONES: 2 o más escalones que explican al menos el 70 % del cambio neto; UN_ESCALON: uno que lo explica; si no, CONTINUO"""
    tot = sum(abs(x['delta']) for x in st)
    if net is None or abs(net) < EPS: return 'CONTINUO'
    if tot >= p['history_step_share'] * abs(net) - EPS:
        return 'ESCALONES' if len(st) >= 2 else ('UN_ESCALON' if len(st) == 1 else 'CONTINUO')
    return 'CONTINUO'


def in_ep(at, episodes):
    """el escalón cae dentro de una alerta de movimiento anterior (desde 12 h antes de abrir hasta el cierre)"""
    return any(e['opened_at'] - 12 * H <= at <= e['cleared_at'] for e in episodes)


def axis_history(lvl, tb, days, i, p, episodes=()):
    a = tb - days * 24 * H
    now = last_level(lvl, tb, i)
    old = lmed(lvl, a - 6 * H, a + 6 * H, i)
    vals = [lvl.v[h][i] for h in lvl.between(a, tb)]
    n_calc = len(lvl.between(a + pd.Timedelta(seconds=1), tb))
    net = None if now is None or old is None else round(now - old, 3)
    rng = round(max(vals) - min(vals), 3) if vals else None
    st = steps(lvl, a, tb, i, p)
    if net is None: pattern = 'SIN_DATOS'
    elif rng is not None and rng <= p['still_range_deg'] + EPS: pattern = 'QUIETO'
    else: pattern = step_pattern(st, net, p)
    return dict(net=net, range=rng, rate_per_day=None if net is None else round(net / days, 4), pattern=pattern,
                steps=len(st), steps_in_alert=sum(1 for s_ in st if in_ep(s_['at'], episodes)),
                step_list=[dict(at=str(s_['at']), delta=s_['delta'], in_alert=in_ep(s_['at'], episodes)) for s_ in st[-5:]],
                coverage=round(n_calc / (days * 24), 2))


def load(path='motion_input.csv'):
    df = pd.read_csv(path)
    df['t'] = pd.to_datetime(df.pop('timestamp'), utc=True)
    return df.set_index('t').sort_index()


def med(df, a, b, col, closed='left'):
    """mediana de col en [a, b) (closed='left') o (a, b] (closed='right'); devuelve (valor, n)"""
    if closed == 'left':
        s = df.loc[(df.index >= a) & (df.index < b), col].dropna()
    else:
        s = df.loc[(df.index > a) & (df.index <= b), col].dropna()
    return (float(s.median()) if len(s) else None), len(s)


def normal_of(p, W, axis):
    for r in p['rules']:
        if r['window_hours'] == W and r['axis'] in ('ANY', axis.upper()): return r['amarillo_deg']
    return None


def cumulative_reason(L, t, long_hits, p):
    hb = max(long_hits, key=lambda h: (ORD[h['level']], abs(h['delta'])))
    i = 0 if hb['axis'] == 'pitch' else 1
    st = steps(L, hb['since'], t, i, p)
    a3 = lmed(L, t - 75 * H, t - 69 * H, i); b3 = lmed(L, t - 3 * H, t, i)
    rate = None if a3 is None or b3 is None else round((b3 - a3) / 3, 4)
    pattern = step_pattern(st, hb['delta'], p)
    return dict(code='MOV_CUMULATIVE', data=dict(axis=hb['axis'], delta=hb['delta'], window_hours=hb['window_hours'],
                since=str(hb['since']), span_hours=hb['span_hours'], level=hb['level'], deg=hb['deg'],
                amarillo_deg=normal_of(p, hb['window_hours'], hb['axis']), steps=len(st),
                step_list=[dict(at=str(x['at']), delta=x['delta']) for x in st[-6:]], pattern=pattern, rate_per_day_3d=rate))


def daily_reason(L, t, S, p):
    if not p['daily_summary']: return None
    el = t - S['episode_opened_at']
    if el <= pd.Timedelta(0) or el % (24 * H) != pd.Timedelta(0): return None
    n24 = len(L.between(t - 23 * H, t))
    if n24 == 0:
        return dict(code='MOV_DAILY', data=dict(no_data=True, calculated_hours_24h=0, day=int(el / (24 * H))))
    d = dict(no_data=False, calculated_hours_24h=n24, day=int(el / (24 * H)), level=S['ep_level'], sent=sent_level(S['ep_level'], p))
    for i, ax in ((0, 'p'), (1, 'r')):
        b = lmed(L, t - 3 * H, t, i); a = lmed(L, t - 27 * H, t - 24 * H, i)
        ref = S['ep_ref_p'] if i == 0 else S['ep_ref_r']
        d[f'd{ax}_24h'] = None if a is None or b is None else round(b - a, 3)
        d[f'd{ax}_total'] = None if b is None or ref is None else round(b - ref, 3)
    return dict(code='MOV_DAILY', data=d)


def run(df, p=P, start=None, end=None):
    pr = df.dropna(subset=['pitch', 'roll'])
    t0 = (start or pr.index.min().ceil('h'))
    t1 = (end or pr.index.max().floor('h'))
    hours = pd.date_range(t0, t1, freq='h')
    ts = pr.index
    # huecos: registro f cuyo anterior está a >= gap_min_hours
    gaps = []
    d = ts.to_series().diff()
    for f, dt in d[d >= pd.Timedelta(hours=p['gap_min_hours'])].items():
        gaps.append((ts[ts.get_loc(f) - 1], f))
    # hora en que se evalúa cada hueco: primera t > f + gap_post_minutes
    gap_at = {}
    for prev, f in gaps:
        tg = (f + pd.Timedelta(minutes=p['gap_post_minutes'])).floor('h') + H
        gap_at.setdefault(tg, []).append((prev, f))

    # medianas horarias (nivel y temperatura) por hora t = mediana de [t-1h, t)
    lvl = {}; jumps = {}; pending_gaps = []; L = Levels(); TH = thresholds(p)
    joint_hours = []; fast_hours = []; episodes = []
    S = dict(displaced=False, ref_p=None, ref_r=None, sign=0, shift_started_at=None, shift_level_p=None,
             shift_level_r=None, shift_confirmed_at=None, gap_from=None, gap_to=None, last_joint_at=None,
             temp_state='TEMP_VARIABLE' if p['temp_mode'] == 'MOVIMIENTO_Y_TEMPERATURA' else None, temp_since=None,
             temp_stable_from=None, temp_stable_to=None, return_started_at=None,
             in_episode=False, episode_opened_at=None, still_run=0, 
             reopen_msgs=0, last_return_at=None, abrupt_active=False,
             # motion-v2
             rest_at=None, hold={k[0]: 0 for k in thresholds(p)}, ep_level='NORMAL', ep_has_long=False,
             ep_ref_p=None, ep_ref_r=None, ep_kind=None, ep_start_at=None, last_pattern_at=None, last_fast_at=None)
    out = []
    for t in hours:
        w = pr[(pr.index >= t - H) & (pr.index < t)]
        n = len(w)
        row = dict(evaluated_at=t, n_records=n, coverage=n / p['records_per_hour'])
        reasons = []
        ev = 'NONE'
        if n < p['min_records_hour']:
            row.update(status='NO_CALCULABLE')
            pending_gaps.extend(gap_at.get(t, []))
            lvl[t] = None
            if S['in_episode']:
                daily = daily_reason(L, t, S, p)
                if daily: reasons.append(daily)
        else:
            row['status'] = 'CALCULATED'
            Pn, Rn = float(w.pitch.median()), float(w.roll.median())
            tw = w.temp.dropna()
            Tn = float(tw.median()) if len(tw) else None
            lvl[t] = (Pn, Rn, Tn); L.add(t, (Pn, Rn, Tn))
            row.update(pitch_now=Pn, roll_now=Rn, temp_now=Tn)
            # dp_1h
            bp, nb = med(pr, t - 3 * H, t - H, 'pitch'); br, _ = med(pr, t - 3 * H, t - H, 'roll')
            dp1 = Pn - bp if nb >= 4 else None; dr1 = Rn - br if nb >= 4 else None
            # base 6h
            Bp, nB = med(pr, t - p['base_from_hours'] * H, t - p['base_to_hours'] * H, 'pitch')
            Br, _ = med(pr, t - p['base_from_hours'] * H, t - p['base_to_hours'] * H, 'roll')
            okB = nB >= p['base_min_records']
            dp6 = Pn - Bp if okB else None; dr6 = Rn - Br if okB else None
            # joint shift records
            w2 = pr[(pr.index >= t - p['shift_window_hours'] * H) & (pr.index < t)]
            jsign = 0
            if okB and len(w2) >= p['shift_min_records']:
                up = ((w2.pitch - Bp >= p['shift_deg']) & (w2.roll - Br >= p['shift_deg'])).all()
                dn = ((w2.pitch - Bp <= -p['shift_deg']) & (w2.roll - Br <= -p['shift_deg'])).all()
                jsign = 1 if up else (-1 if dn else 0)
            row.update(dp_1h=dp1, dr_1h=dr1, dp_6h=dp6, dr_6h=dr6, shift_records_2h=len(w2), shift_sign=jsign)
            # deriva
            der = float(w.deriva.max()) if w.deriva.notna().any() else None
            row['deriva_max_1h'] = der
            J = (der is not None and der >= p['deriva_jump_deg']) or (
                dp1 is not None and dr1 is not None and abs(dp1) >= p['jump_deg'] and abs(dr1) >= p['jump_deg']
                and np.sign(dp1) == np.sign(dr1))
            row['jump'] = bool(J); jumps[t] = bool(J)
            # 24h y 7d
            l24 = lvl.get(t - 24 * H)
            dp24 = Pn - l24[0] if l24 else None; dr24 = Rn - l24[1] if l24 else None
            l7 = lvl.get(t - 168 * H)
            trend = bool(l7 and (abs(Pn - l7[0]) >= p['trend_deg_7d'] or abs(Rn - l7[1]) >= p['trend_deg_7d']))
            row.update(dp_24h=dp24, dr_24h=dr24, trend_flag=trend)
            # temperatura: tasas horarias
            def rate(h):
                a, b = lvl.get(h), lvl.get(h - H)
                if a and b and a[2] is not None and b[2] is not None: return abs(a[2] - b[2])
                return None
            rates6 = [rate(t - k * H) for k in range(p['temp_flat_hours'])]
            r6 = [x for x in rates6 if x is not None]
            flat = len(r6) >= p['temp_flat_min_rates'] and max(r6) < p['temp_flat_rate']
            rl = [rate(t - k * H) for k in range(p['temp_flow_hours'])]
            flow = all(x is not None and x >= p['temp_flow_rate'] for x in rl)
            row.update(temp_rate_max_6h=max(r6) if r6 else None, temp_rate_last=rl[0], temp_flat=flat, temp_flow=flow)

            newly = False
            # 1) hueco
            todo = pending_gaps + [g for g in gap_at.get(t, [])]
            pending_gaps.clear()
            for prev, f in todo:
                pre_p, npre = med(pr, prev - p['gap_pre_hours'] * H, prev, 'pitch', 'right')
                pre_r, _ = med(pr, prev - p['gap_pre_hours'] * H, prev, 'roll', 'right')
                post_p2, npo2 = med(pr, f - pd.Timedelta(seconds=1), f + pd.Timedelta(minutes=p['gap_post_minutes']), 'pitch', 'right')
                if npre >= p['gap_pre_min_records'] and npo2 >= p['gap_post_min_records']:
                    post_r2, _ = med(pr, f - pd.Timedelta(seconds=1), f + pd.Timedelta(minutes=p['gap_post_minutes']), 'roll', 'right')
                    dpg, drg = post_p2 - pre_p, post_r2 - pre_r
                    gsign = 1 if (dpg >= p['shift_deg'] and drg >= p['shift_deg']) else (-1 if (dpg <= -p['shift_deg'] and drg <= -p['shift_deg']) else 0)
                    row.update(gap_from=prev, gap_to=f, dp_gap=dpg, dr_gap=drg)
                    if gsign and not S['displaced']:
                        S.update(displaced=True, ref_p=pre_p, ref_r=pre_r, sign=gsign, shift_started_at=prev,
                                 shift_level_p=post_p2, shift_level_r=post_r2, shift_confirmed_at=t, gap_from=prev, gap_to=f,
                                 last_joint_at=t)
                        reasons.append(dict(code='MOV_SHIFT_GAP', data=dict(dp=round(dpg, 3), dr=round(drg, 3), gap_from=str(prev), gap_to=str(f))))
                        newly = True
                    elif gsign and S['displaced'] and gsign == -S['sign']:
                        pass  # el retorno lo decide la tolerancia (paso 3)
            # 2) desplazamiento sostenido
            sup_ret = S['last_return_at'] is not None and (t - S['last_return_at']) < p['shift_suppress_after_return_hours'] * H
            # el desplazamiento sostenido solo cuenta si fue brusco: hubo un salto (jump) en [t-6h, t]
            jh_any = any(jumps.get(h) for h in pd.date_range(t - 6 * H, t, freq='h'))
            if not S['displaced'] and jsign and not sup_ret and jh_any:
                # inicio: primer registro en [t-6h, t) con los dos ejes >= jump_deg desde la base, en el sentido del salto
                w6 = pr[(pr.index >= t - 6 * H) & (pr.index < t)]
                dv = w6[w6.deriva >= p['deriva_jump_deg']]
                jh = [h for h in pd.date_range(t - 6 * H, t, freq='h') if jumps.get(h)]
                if len(dv):
                    st = dv.index[0]
                elif jh:
                    st = pr[(pr.index >= jh[0] - H) & (pr.index < jh[0])].index[0]
                else:
                    m = (jsign * (w6.pitch - Bp) >= p['jump_deg']) & (jsign * (w6.roll - Br) >= p['jump_deg'])
                    st = w6.index[m.values][0] if m.any() else w.index[0]
                S.update(displaced=True, ref_p=Bp, ref_r=Br, sign=jsign, shift_started_at=st, shift_level_p=Pn,
                         shift_level_r=Rn, shift_confirmed_at=t, gap_from=None, gap_to=None, last_joint_at=t)
                reasons.append(dict(code='MOV_SHIFT', data=dict(dp=round(Pn - Bp, 3), dr=round(Rn - Br, 3), started_at=str(st))))
                newly = True
            # 3) retorno
            returned = False
            if S['displaced'] and not newly:
                sg = S['sign']; f = p['return_fraction']
                recp = sg * (S['shift_level_p'] - Pn); recr = sg * (S['shift_level_r'] - Rn)
                if recp >= f * sg * (S['shift_level_p'] - S['ref_p']) and recr >= f * sg * (S['shift_level_r'] - S['ref_r']):
                    w6 = pr[(pr.index >= t - 6 * H) & (pr.index < t)]
                    sg = S['sign']
                    m = (sg * (S['shift_level_p'] - w6.pitch) >= p['jump_deg']) & (sg * (S['shift_level_r'] - w6.roll) >= p['jump_deg'])
                    rs = w6.index[m.values][0] if m.any() else t - H
                    S.update(displaced=False, return_started_at=rs, last_joint_at=t, last_return_at=t)
                    reasons.append(dict(code='MOV_RETURN', data=dict(dp=round(Pn - S['shift_level_p'], 3), dr=round(Rn - S['shift_level_r'], 3), started_at=str(rs))))
                    returned = True
            if J:
                reasons.append(dict(code='MOV_JUMP', data=dict(dp=None if dp1 is None else round(dp1, 3), dr=None if dr1 is None else round(dr1, 3), deriva=der)))
            # 4) motion-v2: reglas de la tabla
            if S['rest_at'] is None: S['rest_at'] = t
            closure = bool(J or S['displaced'] or newly or returned or (
                S['last_joint_at'] is not None and (t - S['last_joint_at']) <= p['abrupt_suppress_after_joint_hours'] * H))
            hits = []; short_now = False
            for key, W, kind, ax, deg, lev in sorted(TH, key=lambda x: x[1]):
                hit = None
                if kind == 'SHORT':
                    for k in range(1, W + 1):
                        lk = lvl.get(t - k * H)
                        if not lk: continue
                        for an, i in axes(ax):
                            dlt = (Pn, Rn)[i] - lk[i]
                            if abs(dlt) >= deg - EPS and (hit is None or abs(dlt) > abs(hit['delta'])):
                                hit = dict(axis=an, delta=round(dlt, 3), span_hours=k)
                    ok = hit is not None
                else:
                    t0 = max(t - W * H, S['rest_at'])
                    ok = False
                    # lo acumulado mide solo movimiento lento: se omite si en la ventana hubo un patrón conjunto o un cambio rápido
                    joint_in = ((S['last_pattern_at'] is not None and S['last_pattern_at'] >= t0)
                                or (S['last_fast_at'] is not None and S['last_fast_at'] >= t0)
                                or J or newly or returned or bool(short_now))
                    cov = len(L.between(t0 + pd.Timedelta(seconds=1), t)) / max((t - t0) / H, 1)
                    if t0 < t and not joint_in and cov >= p['long_min_coverage'] - EPS:
                        for an, i in axes(ax):
                            ref = lmed(L, t0 - 3 * H, t0 + 3 * H, i)
                            if ref is None: continue
                            dlt = (Pn, Rn)[i] - ref
                            if abs(dlt) >= deg - EPS and (hit is None or abs(dlt) > abs(hit['delta'])):
                                hit = dict(axis=an, delta=round(dlt, 3), since=t0, span_hours=int((t - t0) / H))
                        ok = hit is not None
                    S['hold'][key] = S['hold'][key] + 1 if ok else 0
                    ok = S['hold'][key] >= p['long_hold_hours']
                if ok:
                    hits.append(dict(key=key, window_hours=W, kind=kind, level=lev, deg=deg, **hit))
                    if kind == 'SHORT': short_now = True
            short_hits = [h for h in hits if h['kind'] == 'SHORT']
            long_hits = [h for h in hits if h['kind'] == 'LONG']
            table_level = max([h['level'] for h in hits], key=lambda l: ORD[l], default='NORMAL')
            max_short = max([abs(h['delta']) for h in short_hits], default=0.0)
            if closure and (table_level != 'NORMAL' or J or newly) and max_short <= p['closure_max_deg'] + EPS:
                hour_level = p['closure_cap_level']          # patrón conjunto: tope (cierres de TGI)
            elif J or newly:
                hour_level = table_level if ORD[table_level] >= 1 else 'AMARILLO'
            else:
                hour_level = table_level
            row['table_level_hour'] = hour_level
            row['rules_hit'] = json.dumps([{k: (str(v) if isinstance(v, pd.Timestamp) else v) for k, v in h.items() if k != 'key'} for h in hits])
            # MOV_ABRUPT: inicio de un cambio rápido fuera de un patrón conjunto (como v1)
            abrupt = bool(short_hits) and not closure
            row['abrupt'] = abrupt
            abrupt_fire = False
            if abrupt and not S['abrupt_active']:
                hb = max(short_hits, key=lambda h: (ORD[h['level']], abs(h['delta'])))
                reasons.append(dict(code='MOV_ABRUPT', data=dict(axis=hb['axis'], delta=hb['delta'], hours=hb['span_hours'],
                                                                 window_hours=hb['window_hours'], level=hb['level'], deg=hb['deg'],
                                                                 amarillo_deg=normal_of(p, hb['window_hours'], hb['axis']))))
                abrupt_fire = True
            S['abrupt_active'] = abrupt
            row['slow_drift_flag'] = bool(dp24 is not None and dr24 is not None and
                                          (abs(dp24) >= p['slow_drift_deg_24h'] or abs(dr24) >= p['slow_drift_deg_24h']))
            # 5) quieto
            win = [h for h in pd.date_range(t - (p['still_hours'] - 1) * H, t, freq='h') if lvl.get(h)]
            if len(win) >= p['still_min_calculated'] and not any(jumps.get(h) for h in win):
                pp = [lvl[h][0] for h in win]; rr = [lvl[h][1] for h in win]
                still = (max(pp) - min(pp) <= p['still_range_deg'] + 1e-9) and (max(rr) - min(rr) <= p['still_range_deg'] + 1e-9)
            else:
                still = False
            S['still_run'] = p['still_hours'] if still else 0
            row['still'] = still

            # 6) línea
            if S['temp_state'] is not None:
                ls = S['temp_state']
                if ls == 'TEMP_VARIABLE' and newly:
                    ls = 'VERIFICANDO'; reasons.append(dict(code='TEMP_VERIFYING', data={}))
                elif ls == 'VERIFICANDO':
                    if returned:
                        ls = 'TEMP_VARIABLE'
                    elif flat:
                        ls = 'TEMP_ESTABLE'; S['temp_stable_from'] = S['shift_started_at']; S['temp_stable_to'] = None
                        reasons.append(dict(code='TEMP_STABILIZED', data=dict(temp_rate_max_6h=round(max(r6), 3), temp_stable_from=str(S['shift_started_at']))))
                    elif t - S['shift_started_at'] >= p['stabilize_max_hours'] * H:
                        ls = 'SIN_ESTABILIZAR'; reasons.append(dict(code='TEMP_NOT_STABILIZED', data=dict(hours=p['stabilize_max_hours'])))
                elif ls == 'SIN_ESTABILIZAR':
                    if returned: ls = 'TEMP_VARIABLE'
                if ls == 'TEMP_ESTABLE' and returned:
                    ls = 'RETORNO_TEMP_ESTABLE'; S['reopen_msgs'] = 0; S['temp_since'] = t
                if ls in ('TEMP_ESTABLE', 'RETORNO_TEMP_ESTABLE'):
                    if flow:
                        end_ = S['return_started_at'] if ls == 'RETORNO_TEMP_ESTABLE' else t - p['temp_flow_hours'] * H
                        S['temp_stable_to'] = end_
                        reasons.append(dict(code='TEMP_VARIES_AGAIN', data=dict(rate=round(rl[0], 3), temp_stable_from=str(S['temp_stable_from']), temp_stable_to=str(end_))))
                        ls = 'TEMP_VARIABLE'
                    elif ls == 'RETORNO_TEMP_ESTABLE':
                        hrs = (t - S['temp_since']) / H
                        k = S['reopen_msgs']
                        due = p['still_stable_first_hours'] + k * p['still_stable_every_hours']
                        if hrs >= due:
                            reasons.append(dict(code='TEMP_STILL_STABLE', data=dict(hours=int(hrs), temp_rate_max_6h=round(max(r6), 3) if r6 else None)))
                            S['reopen_msgs'] = k + 1
                if ls != S['temp_state']:
                    S['temp_since'] = t
                S['temp_state'] = ls

            # 7) episodio motion-v2
            if J or newly or returned: S['last_pattern_at'] = t
            if short_hits: S['last_fast_at'] = t
            codes_now = [r['code'] for r in reasons]
            opener = J or newly or hour_level != 'NORMAL'
            if not S['in_episode'] and opener:
                S['in_episode'] = True; S['episode_opened_at'] = t; ev = 'OPENED'
                if 'MOV_SHIFT_GAP' in codes_now:
                    kind, tb = 'HUECO', S['gap_from']
                elif J or 'MOV_SHIFT' in codes_now:
                    kind = 'SALTO'; tb = (S['shift_started_at'] if 'MOV_SHIFT' in codes_now else t - H) - pd.Timedelta(seconds=1)
                elif short_hits:
                    hb = max(short_hits, key=lambda h: (ORD[h['level']], abs(h['delta'])))
                    kind, tb = 'RAPIDO', t - hb['span_hours'] * H
                else:
                    kind, tb = 'ACUMULADO', t
                S['ep_kind'] = kind
                S['ep_level'] = hour_level if hour_level != 'NORMAL' else 'AMARILLO'
                S['ep_has_long'] = bool(long_hits)
                if kind == 'ACUMULADO':
                    hb = max(long_hits, key=lambda h: (ORD[h['level']], abs(h['delta'])))
                    S['ep_start_at'] = hb['since']
                    S['ep_ref_p'] = lmed(L, hb['since'] - 3 * H, hb['since'] + 3 * H, 0)
                    S['ep_ref_r'] = lmed(L, hb['since'] - 3 * H, hb['since'] + 3 * H, 1)
                else:
                    S['ep_start_at'] = tb
                    S['ep_ref_p'] = last_level(L, tb, 0); S['ep_ref_r'] = last_level(L, tb, 1)
                if long_hits:
                    reasons.append(cumulative_reason(L, t, long_hits, p))
                # historial: lo que pasó antes del movimiento (para ACUMULADO incluye la acumulación)
                hist = dict(until=str(tb), kind=kind)
                for D in p['history_days']:
                    hist[f'd{D}'] = dict(pitch=axis_history(L, tb, D, 0, p, episodes), roll=axis_history(L, tb, D, 1, p, episodes))
                hist['normal'] = [dict(window_hours=r['window_hours'], axis=r['axis'], amarillo_deg=r['amarillo_deg'])
                                  for r in p['rules'] if r['window_hours'] in (168, 720)]
                hist['previous_episodes'] = [dict(opened_at=str(e['opened_at']), cleared_at=str(e['cleared_at']), kind=e['kind'], level=e['level'])
                                             for e in episodes if e['opened_at'] >= tb - p['history_previous_days'] * 24 * H][-5:]
                row['history'] = json.dumps(hist)
            elif S['in_episode']:
                if ORD[hour_level] > ORD[S['ep_level']]:
                    cand = [h for h in hits if h['level'] == hour_level]
                    hb = max(cand, key=lambda h: abs(h['delta'])) if cand else None
                    reasons.append(dict(code='MOV_LEVEL_UP', data=dict(
                        from_level=S['ep_level'], to_level=hour_level,
                        sent_from=sent_level(S['ep_level'], p), sent_to=sent_level(hour_level, p),
                        axis=hb and hb['axis'], delta=hb and hb['delta'], window_hours=hb and hb['window_hours'],
                        span_hours=hb and hb['span_hours'], deg=hb and hb['deg'])))
                    S['ep_level'] = hour_level
                if long_hits and not S['ep_has_long']:
                    S['ep_has_long'] = True
                    reasons.append(cumulative_reason(L, t, long_hits, p))
                ok_line = S['temp_state'] in (None, 'TEMP_VARIABLE', 'SIN_ESTABILIZAR')
                net_ok = True
                if S['ep_has_long']:
                    for i in (0, 1):
                        a_ = lmed(L, t - (p['long_close_hours'] + 3) * H, t - (p['long_close_hours'] - 3) * H, i)
                        b_ = lmed(L, t - 3 * H, t, i)
                        if a_ is None or b_ is None or abs(b_ - a_) > p['long_close_net_deg'] + EPS: net_ok = False
                if (S['still_run'] >= p['still_hours'] and ok_line and not short_hits and net_ok
                        and t - S['episode_opened_at'] >= p['still_hours'] * H):
                    S['in_episode'] = False; ev = 'CLEARED'
                    nowp, nowr = lmed(L, t - 3 * H, t, 0), lmed(L, t - 3 * H, t, 1)
                    dpt = None if S['ep_ref_p'] is None else round(nowp - S['ep_ref_p'], 3)
                    drt = None if S['ep_ref_r'] is None else round(nowr - S['ep_ref_r'], 3)
                    moved = (dpt is not None and abs(dpt) >= p['still_range_deg'] - EPS) or (drt is not None and abs(drt) >= p['still_range_deg'] - EPS)
                    if S['displaced'] or moved:
                        S.update(displaced=False)
                        reasons.append(dict(code='MOV_NEW_POSITION', data=dict(dp_total=dpt, dr_total=drt)))
                    if S['temp_state'] == 'SIN_ESTABILIZAR': S['temp_state'] = 'TEMP_VARIABLE'; S['temp_since'] = t
                    reasons.append(dict(code='MOV_STILL', data=dict(hours=S['still_run'], dp_total=dpt, dr_total=drt,
                                        started_at=str(S['ep_start_at']), max_level=S['ep_level'], max_sent=sent_level(S['ep_level'], p),
                                        long_episode=S['ep_has_long'])))
                    episodes.append(dict(opened_at=S['episode_opened_at'], cleared_at=t, kind=S['ep_kind'], level=S['ep_level']))
                    S['rest_at'] = t
                    for k in S['hold']: S['hold'][k] = 0
                else:
                    daily = daily_reason(L, t, S, p)
                    if daily: reasons.append(daily)
        # estado del movimiento
        codes = [r['code'] for r in reasons]
        if row['status'] == 'NO_CALCULABLE':
            ms = 'DESPLAZADO' if S['displaced'] else None
        elif S['displaced']:
            ms = 'DESPLAZADO'
        elif 'MOV_RETURN' in codes:
            ms = 'RETORNO'
        elif 'MOV_JUMP' in codes:
            ms = 'SALTO'
        elif S['abrupt_active']:
            ms = 'BRUSCO'
        else:
            ms = 'NORMAL'
        row.update(motion_state=ms, temp_state=S['temp_state'], in_episode=S['in_episode'], episode_event=ev,
                   computed_level=sent_level(S['ep_level'], p) if S['in_episode'] else 'NORMAL',
                   table_level=S['ep_level'] if S['in_episode'] else 'NORMAL',
                   episode_kind=S['ep_kind'] if S['in_episode'] or ev == 'CLEARED' else None,
                   rest_at=S['rest_at'], last_pattern_at=S['last_pattern_at'], last_fast_at=S['last_fast_at'],
                   long_hold=json.dumps({k: v for k, v in S['hold'].items() if v}),
                   ep_start_at=S['ep_start_at'] if S['in_episode'] else None,
                   ep_ref_pitch=S['ep_ref_p'] if S['in_episode'] else None, ep_ref_roll=S['ep_ref_r'] if S['in_episode'] else None,
                   ep_has_long=S['ep_has_long'] if S['in_episode'] else False,
                   reference_pitch=S['ref_p'] if S['displaced'] else None, reference_roll=S['ref_r'] if S['displaced'] else None,
                   shift_started_at=S['shift_started_at'] if S['displaced'] else None,
                   temp_stable_from=S['temp_stable_from'] if S['temp_state'] in ('TEMP_ESTABLE', 'RETORNO_TEMP_ESTABLE') or 'TEMP_VARIES_AGAIN' in codes else None,
                   temp_stable_to=S['temp_stable_to'] if 'TEMP_VARIES_AGAIN' in codes else None,
                   reasons=json.dumps(reasons, default=str))
        out.append(row)
    return pd.DataFrame(out)


if __name__ == '__main__':
    import sys
    INP = sys.argv[1] if len(sys.argv) > 1 else 'motion_input.csv'
    OUT = sys.argv[2] if len(sys.argv) > 2 else 'motion_v2_expected.csv'
    df = load(INP)
    R = run(df)
    for c in R.columns:
        if str(R[c].dtype).startswith('datetime') or R[c].map(lambda v: isinstance(v, pd.Timestamp)).any():
            R[c] = R[c].map(lambda v: v.strftime('%Y-%m-%dT%H:%M:%SZ') if isinstance(v, pd.Timestamp) else v)
    import re
    txt = R.to_csv(index=False)
    txt = re.sub(r'(\d{4}-\d\d-\d\d)[ T](\d\d:\d\d:\d\d)\+00:00', r'\1T\2Z', txt)
    open(OUT, 'w').write(txt)
    pd.set_option('display.width', 250); pd.set_option('display.max_colwidth', 200)
    ev = R[(R.reasons != '[]') | (R.episode_event != 'NONE')]
    loc = lambda s: pd.to_datetime(s).dt.tz_convert('America/Bogota').dt.strftime('%d %b %H:%M')
    E = ev.assign(hora_col=loc(ev.evaluated_at))[['hora_col', 'evaluated_at', 'status', 'motion_state', 'temp_state', 'episode_event', 'reasons']]
    print(E.to_string())
    print(R.status.value_counts())
