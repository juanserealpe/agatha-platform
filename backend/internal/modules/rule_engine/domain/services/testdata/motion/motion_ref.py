"""AGATHA · Implementación de referencia de MotionAssess (Rule Engine, MOVIMIENTO).
Reproduce la especificación del requerimiento hora a hora, en UTC, sobre raw_measurements
(pitch=1, roll=2, deriva=19, temperatura=20). Uso: python3 motion_ref.py  (lee motion_input.csv, escribe motion_expected.csv)
Hora t = fin de la ventana: la hora t cubre [t-1h, t), como feature_evaluations."""
import pandas as pd, numpy as np, json

P = dict(  # motion_threshold_versions v1 del MP-11556
    records_per_hour=4, min_records_hour=3,
    jump_deg=0.03, deriva_jump_deg=0.03,
    shift_deg=0.04, shift_window_hours=2, shift_min_records=6,
    base_from_hours=24, base_to_hours=6, base_min_records=36,
    gap_min_hours=1.0, gap_pre_hours=2, gap_post_minutes=45, gap_pre_min_records=3, gap_post_min_records=2,
    return_fraction=0.5, shift_suppress_after_return_hours=24,
    abrupt_deg=0.04, abrupt_hours=3, abrupt_suppress_after_joint_hours=6, slow_drift_deg_24h=0.04,
    still_hours=24, still_min_calculated=20, still_range_deg=0.02,
    temp_flat_rate=0.03, temp_flat_hours=6, temp_flat_min_rates=5,
    temp_flow_rate=0.08, temp_flow_hours=2,
    stabilize_max_hours=36,
    still_stable_first_hours=6, still_stable_every_hours=12,
    trend_deg_7d=0.04,
    temp_mode='MOVIMIENTO_Y_TEMPERATURA')

H = pd.Timedelta('1h')


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
    lvl = {}; jumps = {}; pending_gaps = []
    S = dict(displaced=False, ref_p=None, ref_r=None, sign=0, shift_started_at=None, shift_level_p=None,
             shift_level_r=None, shift_confirmed_at=None, gap_from=None, gap_to=None, last_joint_at=None,
             temp_state='TEMP_VARIABLE' if p['temp_mode'] == 'MOVIMIENTO_Y_TEMPERATURA' else None, temp_since=None,
             temp_stable_from=None, temp_stable_to=None, return_started_at=None,
             in_episode=False, episode_opened_at=None, still_run=0, 
             reopen_msgs=0, last_return_at=None, abrupt_active=False)
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
        else:
            row['status'] = 'CALCULATED'
            Pn, Rn = float(w.pitch.median()), float(w.roll.median())
            tw = w.temp.dropna()
            Tn = float(tw.median()) if len(tw) else None
            lvl[t] = (Pn, Rn, Tn)
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
            # 4) cambio brusco (un eje o los dos) que no es un desplazamiento conjunto
            abrupt_fire = False
            best = None
            for k in range(1, p['abrupt_hours'] + 1):
                lk = lvl.get(t - k * H)
                if not lk: continue
                for ax, cur, prv in (('pitch', Pn, lk[0]), ('roll', Rn, lk[1])):
                    dlt = cur - prv
                    if abs(dlt) >= p['abrupt_deg'] - 1e-9 and (best is None or abs(dlt) > abs(best[1])):
                        best = (ax, dlt, k)
            sup = S['last_joint_at'] is not None and (t - S['last_joint_at']) <= p['abrupt_suppress_after_joint_hours'] * H
            abrupt = best is not None and not J and not S['displaced'] and not newly and not sup
            row['abrupt'] = abrupt
            if abrupt and not S['abrupt_active']:
                reasons.append(dict(code='MOV_ABRUPT', data=dict(axis=best[0], delta=round(best[1], 3), hours=best[2])))
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

            # 7) episodio
            opener = J or newly or abrupt_fire
            if not S['in_episode'] and opener:
                S['in_episode'] = True; S['episode_opened_at'] = t; ev = 'OPENED'
            elif S['in_episode']:
                ok_line = S['temp_state'] in (None, 'TEMP_VARIABLE', 'SIN_ESTABILIZAR')
                if (S['still_run'] >= p['still_hours'] and ok_line
                        and t - S['episode_opened_at'] >= p['still_hours'] * H):
                    S['in_episode'] = False; ev = 'CLEARED'
                    if S['displaced']:
                        S.update(displaced=False)
                        reasons.append(dict(code='MOV_NEW_POSITION', data={}))
                    if S['temp_state'] == 'SIN_ESTABILIZAR': S['temp_state'] = 'TEMP_VARIABLE'; S['temp_since'] = t
                    reasons.append(dict(code='MOV_STILL', data=dict(hours=S['still_run'])))
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
                   computed_level='AMARILLO' if S['in_episode'] else 'NORMAL',
                   reference_pitch=S['ref_p'] if S['displaced'] else None, reference_roll=S['ref_r'] if S['displaced'] else None,
                   shift_started_at=S['shift_started_at'] if S['displaced'] else None,
                   temp_stable_from=S['temp_stable_from'] if S['temp_state'] in ('TEMP_ESTABLE', 'RETORNO_TEMP_ESTABLE') or 'TEMP_VARIES_AGAIN' in codes else None,
                   temp_stable_to=S['temp_stable_to'] if 'TEMP_VARIES_AGAIN' in codes else None,
                   reasons=json.dumps(reasons, default=str))
        out.append(row)
    return pd.DataFrame(out)


if __name__ == '__main__':
    df = load()
    R = run(df)
    for c in R.columns:
        if str(R[c].dtype).startswith('datetime') or R[c].map(lambda v: isinstance(v, pd.Timestamp)).any():
            R[c] = R[c].map(lambda v: v.strftime('%Y-%m-%dT%H:%M:%SZ') if isinstance(v, pd.Timestamp) else v)
    import re
    txt = R.to_csv(index=False)
    txt = re.sub(r'(\d{4}-\d\d-\d\d)[ T](\d\d:\d\d:\d\d)\+00:00', r'\1T\2Z', txt)
    open('motion_expected.csv', 'w').write(txt)
    pd.set_option('display.width', 250); pd.set_option('display.max_colwidth', 200)
    ev = R[(R.reasons != '[]') | (R.episode_event != 'NONE')]
    loc = lambda s: pd.to_datetime(s).dt.tz_convert('America/Bogota').dt.strftime('%d %b %H:%M')
    E = ev.assign(hora_col=loc(ev.evaluated_at))[['hora_col', 'evaluated_at', 'status', 'motion_state', 'temp_state', 'episode_event', 'reasons']]
    print(E.to_string())
    print(R.status.value_counts())
