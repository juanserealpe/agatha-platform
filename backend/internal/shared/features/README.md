# Kernel de cálculo (`shared/features`)

Fórmulas de la línea base y del índice I, compartidas por **Baseline** y **Processing** (`WindowEvaluator`). Ningún módulo reimplementa fórmulas: las consume de acá.

**Reglas**: solo stdlib (`depguard` `features-stdlib-only`), sin IO, sin estado, sin tipos de módulos. Todo cambio de fórmula, piso o constante sube `AlgorithmVersion` (hoy `1.0.0`), que se persiste en cada versión de línea base.

## Constantes

`Slot = 15m`, `MaxGap = 20m`, `CoverageMinPct = 70`, `MinCoherenceSamples = 40`, `PFloorHours = 0.25`, `CFloor = 0.01`, `MADScale = 1.4826`, `SigmoidK = 0.6`, `SigmoidZ0 = 2`. `ExpectedSamples(h)` = 24 (6 h) / 96 (24 h).

## API

| Función | Qué hace |
|---|---|
| `Normalize([]Sample)` | Ordena, un sample por slot de 15 min (prefiere B2 válido; si empatan, el más temprano), NaN/±Inf ⇒ ausente |
| `Quantile(xs, q)` | Tipo 7 (interpolación lineal, = pandas/numpy) |
| `Median`, `MAD` | MAD crudo `mediana(|x − mediana|)` |
| `Spearman(x, y)` | Rangos promedio; serie constante ⇒ `(0, true)` |
| `ComputeComponents(window, h, sig, withC)` | Cobertura, D, P, Ex, C, D_Pl |
| `MADFloor(component, h, sig)` | P = 0,25 h; D = 100/N_esp; Ex = máx(min_elevado − mediana, Δmin); C = 0,01 |
| `Score(c, stats, h, sig)` | z, s(z), I; `false` si falta C o alguna estadística |

## Fórmulas

| Variable | Definición |
|---|---|
| Cobertura | `100 × NValid / NExpected` (tope 100); calculable si ≥ 70 |
| D | `100 × N(B2 > θ_D) / NValid` (estricto) |
| P | Máxima duración (h) de rachas de elevados; corta un no elevado o > 20 min entre samples con B2 válido; racha de 1 = 0 h |
| Ex | `Σ máx(0, B2 − mediana) × NExpected / NValid` |
| C | `(|ρ(B1,B2)| + |ρ(B1,Pl)| + |ρ(B2,Pl)|) / 3`, ≥ 40 samples completos, par constante aporta 0 |
| D_Pl | `100 × N(Pl > P90_Pl) / N(Pl)` (informativa) |
| z | `(X − mediana) / (máx(MAD, piso) × 1,4826)` |
| s(z) | `1 / (1 + e^(−0,6 (z − 2)))` |
| I | `s(P) × [s(D) + s(Ex) + s(C)] / 3` |
