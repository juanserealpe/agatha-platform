"""AGATHA · Referencia de los mensajes de alertas de MOVIMIENTO (Alert Management, catálogo v8).
Lee la salida de motion_v2_ref.py y arma, hora por hora, las alertas de MOVIMIENTO y sus mensajes,
como lo haría DecideMotion v8 con la política vigente (reactivación < 24 h si está Sin clasificar).
Uso: python3 messages_v2_ref.py motion_v2_expected.csv [salida.json]"""
import sys, json
import pandas as pd

TZ = 'America/Bogota'
H = pd.Timedelta('1h')
ORD = {'NORMAL': 0, 'AMARILLO': 1, 'NARANJA': 2, 'ROJO': 3}
NIV = {'AMARILLO': 'Amarillo', 'NARANJA': 'Naranja', 'ROJO': 'Rojo'}


def T(x):
    return x if isinstance(x, pd.Timestamp) else pd.Timestamp(x)


def hora(x): return T(x).tz_convert(TZ).strftime('%H:%M')
def fecha(x): return T(x).tz_convert(TZ).strftime('%d/%m')
def hf(x): return f"{hora(x)} del {fecha(x)}"


from decimal import Decimal, ROUND_HALF_UP


def r2(v, q='0.01'):
    """redondeo medio alejándose de cero (0,105 -> 0,11), sobre el valor guardado con 3 decimales"""
    return float(Decimal(str(round(float(v), 6))).quantize(Decimal(q), rounding=ROUND_HALF_UP))


def g(v):
    """grados con signo y dos decimales; 0,00° sin signo"""
    if v is None: return 's/d'
    v = r2(v)
    if abs(v) < 0.005: return '0,00°'
    return f"{v:+.2f}".replace('.', ',') + '°'


def g0(v):  # grados sin signo
    return f"{abs(r2(v)):.2f}".replace('.', ',') + '°'


def rate(v):
    v = r2(v, '0.001')
    return (f"{v:+.3f}".replace('.', ',') if abs(v) >= 0.0005 else '0,000') + '° por día'


def ventana(W):
    if W < 24: return '1 hora' if W == 1 else f'{W} horas'
    d = W // 24
    return '1 día' if d == 1 else f'{d} días'


def dur(h):
    h = int(h + 0.5); d, r = divmod(h, 24)   # medio hacia arriba
    if d == 0: return f"{r} h"
    s = f"{d} día" + ('s' if d > 1 else '')
    return s + (f" y {r} h" if r else '')


def lista(xs):
    return xs[0] if len(xs) == 1 else ', '.join(xs[:-1]) + ' y ' + xs[-1]



def entre(a, b):
    if fecha(a) == fecha(b): return f"Entre las {hora(a)} y las {hora(b)} del {fecha(b)}"
    return f"Entre las {hora(a)} del {fecha(a)} y las {hora(b)} del {fecha(b)}"


KIND_TXT = {'SALTO': 'cambio brusco de pitch y roll', 'HUECO': 'cambio durante pérdida de datos',
            'RAPIDO': 'cambio rápido', 'ACUMULADO': 'cambio lento acumulado'}


def axis_txt(ax, x):
    pat = x['pattern']
    if pat == 'SIN_DATOS': return f"{ax} sin datos suficientes"
    if pat == 'QUIETO': return f"{ax} estable ({g(x['net'])})"
    if pat == 'CONTINUO': return f"{ax} {g(x['net'])}, de forma continua"
    uniq = lambda xs: list(dict.fromkeys(xs))
    con = uniq([fecha(s['at']) for s in x['step_list'] if s['in_alert']])
    sin = uniq([fecha(s['at']) for s in x['step_list'] if not s['in_alert']])
    det = []
    if con: det.append('con alerta: ' + lista(con))
    if sin: det.append(('sin alerta: ' if con else '') + lista(sin))
    n = x['steps']
    cuantos = 'en un cambio' if n == 1 else f'en {n} cambios'
    if n > len(x['step_list']): det[0] = 'los últimos, ' + det[0]
    return f"{ax} {g(x['net'])}, {cuantos} ({'; '.join(det)})"


def history_block(h):
    x7, x30 = h['d7'], h['d30']
    s = (f"Antes de esta alerta. Últimos 7 días: {axis_txt('pitch', x7['pitch'])}; {axis_txt('roll', x7['roll'])}. "
         f"Últimos 30 días: {axis_txt('pitch', x30['pitch'])}; {axis_txt('roll', x30['roll'])}.")
    nm = {n['window_hours']: n['amarillo_deg'] for n in h['normal']}
    if nm:
        ns = [f"{g0(nm[w])} en {ventana(w)}" for w in sorted(nm)]
        s += f" Lo normal en este sensor: menos de {lista(ns)}."
    prev = h['previous_episodes']
    if prev:
        it = [f"{fecha(e['opened_at'])} {KIND_TXT[e['kind']]}" for e in prev]
        s += f" Alertas de movimiento en los últimos 90 días: {len(prev)} ({'; '.join(it)})."
    else:
        s += ' Sin alertas de movimiento en los últimos 90 días.'
    falt = [(D, h[f'd{D}']['pitch']['coverage']) for D in (7, 30) if h[f'd{D}']['pitch']['coverage'] < 0.8]
    for D, c in falt:
        s += f" En los últimos {D} días faltan datos en el {round((1 - c) * 100)} % de las horas."
    return s


def calc_txt(calc, sent):
    if ORD[calc] > ORD[sent]:
        return (f" Por su magnitud correspondería a {NIV[calc]}; ese nivel todavía no está activo para este sensor,"
                f" por eso la alerta se emite en {NIV[sent]}.")
    return ''


def open_msg(t, row, R, hist):
    kind = row['episode_kind']; codes = {r['code']: r['data'] for r in R}
    calc, sent = row['table_level'], row['computed_level']
    if kind == 'SALTO':
        j = codes.get('MOV_JUMP') or {}
        title = 'Cambio brusco de inclinación del sensor en pitch y roll'
        body = (f"{entre(t - H, t)} el pitch y el roll cambiaron juntos y de golpe: "
                f"pitch {g(j.get('dp'))}, roll {g(j.get('dr'))} en 1 h. En este sensor lo normal es menos de 0,01° por hora.")
    elif kind == 'HUECO':
        d = codes['MOV_SHIFT_GAP']
        hs = (T(d['gap_to']) - T(d['gap_from'])) / H
        title = 'Cambio de inclinación del sensor durante una pérdida de datos'
        body = (f"El sensor no transmitió entre las {hf(d['gap_from'])} y las {hf(d['gap_to'])} ({dur(hs)}). "
                f"Al volver los datos, el pitch estaba {g(d['dp'])} y el roll {g(d['dr'])} respecto a antes de la pérdida de datos.")
    elif kind == 'RAPIDO':
        d = codes['MOV_ABRUPT']
        a = t - d['hours'] * H
        title = 'Cambio rápido de inclinación del sensor'
        body = (f"{entre(a, t)} el {d['axis']} cambió {g(d['delta'])} "
                f"({ventana(d['hours'])}). En este sensor lo normal es menos de {g0(d['amarillo_deg'])} en {ventana(d['window_hours'])}.")
    else:
        d = codes['MOV_CUMULATIVE']
        title = 'Cambio lento y acumulado de inclinación del sensor'
        body = cumulative_sentence(t, d)
    body += calc_txt(calc, sent)
    body += '\n' + history_block(hist)
    return title, body


def cumulative_sentence(t, d):
    s = (f"El {d['axis']} acumula {g(d['delta'])} en {ventana(d['span_hours'])} (del {fecha(d['since'])} al {fecha(t)}")
    if d['pattern'] == 'ESCALONES':
        fs = list(dict.fromkeys(fecha(x['at']) for x in d['step_list']))
        s += f"), en {d['steps']} cambios pequeños ({lista(fs)})."
    else:
        s += ', de forma continua'
        s += f"; en los últimos 3 días, {rate(d['rate_per_day_3d'])})." if d.get('rate_per_day_3d') is not None else ')'
    s += f" En este sensor lo normal es menos de {g0(d['amarillo_deg'])} en {ventana(d['window_hours'])}."
    return s


def run(csv):
    R = pd.read_csv(csv)
    alerts = []; act = None; last_fin = None; n = 0
    prev_shift_start = None
    for _, row in R.iterrows():
        t = T(row['evaluated_at']); reasons = json.loads(row['reasons']); codes = [r['code'] for r in reasons]
        data = {r['code']: r['data'] for r in reasons}
        msgs = []
        if row['episode_event'] == 'OPENED':
            hist = json.loads(row['history'])
            kind = row['episode_kind']
            if kind == 'RAPIDO': st = t - (data['MOV_ABRUPT']['hours'] - 1) * H
            elif kind == 'HUECO': st = T(data['MOV_SHIFT_GAP']['gap_from']).floor('h') + H
            else: st = t
            if last_fin is not None and t - last_fin['ended_at'] < 24 * H:
                act = last_fin; act['reactivations'] += 1; act['ended_at'] = None; act['segment_open'] = t
            else:
                n += 1
                act = dict(id=f'MOV-{n}', started_at=st, opened_at=t, ended_at=None, reactivations=0, level=row['computed_level'],
                           calc=row['table_level'], messages=[], segment_open=t, pause_h=0.0, kind=kind)
                alerts.append(act)
            act['level'] = row['computed_level']; act['calc'] = row['table_level']; act['ep_kind'] = kind
            title, body = open_msg(t, row, reasons, hist)
            msgs.append(('MOV_OPEN', 'NOTIFICACION', title, body))
        elif act is not None and act['ended_at'] is None:
            for c in ('MOV_SHIFT', 'MOV_SHIFT_GAP', 'MOV_ABRUPT'):
                if c in data:
                    d = data[c]
                    if c == 'MOV_ABRUPT':
                        msgs.append(('MOV_DISPLACED', 'ACTUALIZACION', 'El sensor se volvió a mover',
                                     f"{entre(t - d['hours'] * H, t)} el {d['axis']} cambió {g(d['delta'])}."))
                    else:
                        st_ = d.get('started_at') or d.get('gap_from')
                        msgs.append(('MOV_DISPLACED', 'ACTUALIZACION', 'El sensor sigue en la nueva posición',
                                     f"No ha vuelto a su posición desde las {hf(st_)}. Pitch {g(d['dp'])}, roll {g(d['dr'])}."))
            if 'MOV_LEVEL_UP' in data:
                d = data['MOV_LEVEL_UP']
                cuando = ventana(d['span_hours'])
                if ORD[d['sent_to']] > ORD[d['sent_from']]:
                    act['level'] = d['sent_to']
                    msgs.append(('MOV_LEVEL_UP', 'NOTIFICACION', f"La alerta de movimiento subió a {NIV[d['sent_to']]}",
                                 f"El {d['axis']} ya cambió {g(d['delta'])} en {cuando}. En este sensor, {NIV[d['to_level']]} es desde "
                                 f"{g0(d['deg'])} en {ventana(d['window_hours'])}."))
                else:
                    msgs.append(('MOV_LEVEL_OBSERVED', 'ACTUALIZACION', 'El cambio de inclinación sigue aumentando',
                                 f"El {d['axis']} ya cambió {g(d['delta'])} en {cuando}. Por su magnitud correspondería a {NIV[d['to_level']]} "
                                 f"(desde {g0(d['deg'])} en {ventana(d['window_hours'])}); ese nivel todavía no está activo para este sensor, "
                                 f"por eso la alerta sigue en nivel {NIV[act['level']]}."))
                act['calc'] = d['to_level']
        if act is not None and act['ended_at'] is None and row['episode_event'] != 'OPENED':
            if 'TEMP_STABILIZED' in data:
                d = data['TEMP_STABILIZED']
                msgs.append(('TEMP_STABILIZED', 'NOTIFICACION', 'La temperatura de la tubería se estabilizó',
                             f"Desde que se movió el sensor ({hf(d['temp_stable_from'])}) la temperatura casi no cambia. Pasó lo mismo antes "
                             f"en este punto, pero no está confirmado. Dato: máximo {str(d['temp_rate_max_6h']).replace('.', ',')} °C por hora (normal: más de 0,08)."))
            if 'MOV_RETURN' in data:
                d = data['MOV_RETURN']
                msgs.append(('MOV_RETURN', 'ACTUALIZACION', 'El sensor volvió a su posición',
                             f"Regresó a las {hora(d['started_at'])} a la posición que tenía antes del movimiento. Pitch {g(d['dp'])}, roll {g(d['dr'])}."))
            if 'TEMP_VARIES_AGAIN' in data:
                d = data['TEMP_VARIES_AGAIN']
                hs = (T(d['temp_stable_to']) - T(d['temp_stable_from'])) / H
                msgs.append(('TEMP_VARIES_AGAIN', 'NOTIFICACION', 'La temperatura de la tubería volvió a variar',
                             f"Estuvo estable {dur(hs)} ({hf(d['temp_stable_from'])} – {hf(d['temp_stable_to'])}). Dato: {str(d['rate']).replace('.', ',')} °C en una hora."))
                act['tramo'] = (d['temp_stable_from'], d['temp_stable_to'])
            if 'MOV_CUMULATIVE' in data:
                d = data['MOV_CUMULATIVE']
                msgs.append(('MOV_SLOW_ONGOING', 'ACTUALIZACION', 'Además, cambio lento y acumulado de inclinación', cumulative_sentence(t, d)))
            if 'MOV_DAILY' in data:
                d = data['MOV_DAILY']
                title = f"Resumen del día {d['day']} de la alerta de movimiento"
                if d['no_data']:
                    body = 'Sin datos del sensor en las últimas 24 h.'
                elif d['dp_24h'] is None or d['dr_24h'] is None:
                    body = f"Con datos en {d['calculated_hours_24h']} de las últimas 24 h; no alcanza para calcular el cambio del día."
                    if d['dp_total'] is not None:
                        body += f" Desde el inicio del movimiento: pitch {g(d['dp_total'])}, roll {g(d['dr_total'])}."
                else:
                    quieto = abs(d['dp_24h']) < 0.01 and abs(d['dr_24h']) < 0.01
                    body = (f"Últimas 24 h: pitch {g(d['dp_24h'])}, roll {g(d['dr_24h'])}. Desde el inicio del movimiento: "
                            f"pitch {g(d['dp_total'])}, roll {g(d['dr_total'])}."
                            + (' Sin cambios en las últimas 24 h.' if quieto else ''))
                    if d['calculated_hours_24h'] < 20:
                        body += f" Con datos en {d['calculated_hours_24h']} de las 24 h."
                msgs.append(('MOV_DAILY', 'RESUMEN', title, body))
            if row['episode_event'] == 'CLEARED':
                d = data['MOV_STILL']
                quiet = '3 días sin cambio neto' if d['long_episode'] else '24 h sin moverse'
                seg_h = (t - act['segment_open']) / H if act['reactivations'] else (t - act['started_at']) / H
                body = f"La inclinación lleva {quiet}. Duró {dur(seg_h)}"
                if act['reactivations']:
                    body += f" desde que se reactivó; la alerta en total, {dur((t - act['started_at']) / H)}"
                body += (f". Cambio total desde el inicio del movimiento: pitch {g(d['dp_total'])}, roll {g(d['dr_total'])}. "
                         f"Nivel máximo de la alerta: {NIV[act['level']]}")
                if ORD[d['max_level']] > ORD[act['level']]:
                    body += f" (por su magnitud correspondía a {NIV[d['max_level']]})"
                body += '.'
                if act.get('tramo'):
                    a, b = act['tramo']; body += f" Temperatura estable del {hf(a)} al {hf(b)}."
                if 'MOV_NEW_POSITION' in data:
                    body += ' La posición actual queda como la nueva referencia del sensor.'
                msgs.append(('END_NORMAL', 'FIN', 'La alerta de movimiento terminó', body))
                act['ended_at'] = t; last_fin = act
        for c, k, ti, b in msgs:
            act['messages'].append(dict(evaluated_at=str(t), hora_col=t.tz_convert(TZ).strftime('%d/%m %H:%M'), code=c, kind=k, title=ti, body=b))
    out = []
    for a in alerts:
        out.append(dict(alert=a['id'], started_at=str(a['started_at']), ended_at=str(a['ended_at']), reactivations=a['reactivations'],
                        max_level=a['level'], max_calc=a['calc'], messages=a['messages']))
    return out


if __name__ == '__main__':
    src = sys.argv[1]; dst = sys.argv[2] if len(sys.argv) > 2 else src.replace('motion_v2_expected', 'alert_messages_v2_expected').replace('.csv', '.json')
    out = run(src)
    json.dump(out, open(dst, 'w'), ensure_ascii=False, indent=1)
    for a in out:
        print(f"\n## {a['alert']} · {a['started_at']} → {a['ended_at']} · enviado {a['max_level']} · calculado {a['max_calc']} · reactivaciones {a['reactivations']}")
        for m in a['messages']:
            print(f"- {m['hora_col']} [{m['code']}/{m['kind']}] **{m['title']}** — {m['body']}")
