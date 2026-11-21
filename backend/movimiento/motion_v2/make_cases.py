"""Genera las entradas sintéticas (datos reales + movimiento sumado) para las pruebas doradas motion-v2."""
import pandas as pd, numpy as np
TZ = 'America/Bogota'
def ts(s): return pd.Timestamp(s, tz=TZ).tz_convert('UTC')
df = pd.read_csv('motion_input.csv')
t = pd.to_datetime(df.timestamp, utc=True)
def lin(a, b, total):  # rampa lineal de 0 a total entre a y b, redondeada a 0,01 (resolución del equipo)
    f = ((t - a) / (b - a)).clip(0, 1).astype(float)
    return np.round(total * f, 2)
CASES = {
  'creep':        ('pitch', lin(ts('2026-11-01 00:00'), ts('2026-11-09 00:00'), -0.24)),
  'escalones':    ('roll',  sum(0.02 * (t >= ts('2026-09-05 02:00') + pd.Timedelta(days=d)) for d in range(6))),
  'rapido_grande':('roll',  lin(ts('2026-09-10 14:00'), ts('2026-09-10 16:00'), 0.12)),
  'rapido_rojo':  ('pitch', -0.25 * (t >= ts('2026-09-15 09:10'))),
}
for name, (col, add) in CASES.items():
    d = df.copy(); m = d[col].notna()
    d.loc[m, col] = np.round(d.loc[m, col] + np.asarray(add)[m.values], 4)
    d.to_csv(f'motion_input_{name}.csv', index=False)
    print(name, col, float(np.asarray(add).min()), float(np.asarray(add).max()))
