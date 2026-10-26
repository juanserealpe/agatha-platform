# Modulo: Baseline (Device Context & Baseline)

**Responsabilidad**: mantener, por dispositivo y **versionada en el tiempo**, la línea base estadística de su comportamiento normal.

- **Señal**: `mediana_B2`, `θ_D = P90_B2`, `P90_Planitud`, `min_elevado_B2`, `Δmin`.
- **Componentes**: mediana y MAD crudo de D, P, Ex (ventanas 6 h y 24 h) y C (solo 24 h).

Es **autosuficiente**: solo lee `raw_measurements` y `devices` (escritor: Ingestion). No publica ni consume eventos. Todo en **UTC** (`America/Bogota` solo en logs).

## Estructura

```
baseline/
├── domain/
│   ├── errors.go
│   ├── entities/                     # BaselineVersion, SignalBaseline, ComponentStat, ExcludedPeriod,
│   │                                 # ExclusionSet, EstimationSummary, PassReport, enums
│   ├── services/                     # Servicios puros (sin IO)
│   └── repositories/                 # Ports: HistoryReader, BaselineRepository, KnownEventsProvider
├── application/services/             # UC-B1/UC-B2 RecalculationService, UC-B4 BackfillService,
│                                     # ReplayService (simulación)
└── infrastructure/
    ├── persistence/                  # models (GORM), mappers, repositories
    ├── knownevents/                  # StaticProvider (lista en código) + CompositeProvider (une fuentes)
    └── scheduling/recalc_scheduler.go
```

## Casos de uso

| ID | Servicio | Invocado por |
|---|---|---|
| UC-B1 | `RecalculationService.Recalculate(RecalculateCommand{PointID, AsOf, Trigger, DryRun})` | UC-B2, `cmd/baseline` |
| UC-B2 | `RecalculationService.RecalculateDue(now)` — secuencial; un punto que falla no detiene al resto | `RecalcScheduler` |
| UC-B4 | `BackfillService.Backfill(BackfillCommand{PointID, From, To, DryRun})` — histórico retroactivo de versiones | `cmd/baseline -backfill` |

UC-B1: valida punto → `GetSamples([asOf − 90 d, asOf))` → `GetKnownEvents` del mismo rango (si falla ⇒ `ErrConsultingKnownEvents`, no se calcula) → `features.Normalize` → `RobustBaselineEstimator.EstimateExcluding` → `INSUFFICIENT_HISTORY` no persiste (log `Info` solo si cambia el motivo) → `DryRun` no persiste → `SaveNewVersion` (conflicto concurrente = no-op).

UC-B4: valida punto → (salvo `DryRun`) `GetLatest` y, si el punto **ya tiene versiones**, `ErrBaselineHistoryExists` → `From` cero ⇒ `GetFirstSeenAt` (sin datos ⇒ `INSUFFICIENT_HISTORY`) → `ReplayService.SimulateVersions(from, to)` → sin versiones ⇒ `INSUFFICIENT_HISTORY` → `DryRun` no persiste → `SaveHistory` (una TX, todo o nada). Ver "Histórico retroactivo".

## Algoritmo (resumen)

1. **Estimación iterativa** (máx. 5 pasadas): calcula la base con el conjunto excluido `E_{k−1}`, barre I(24 h) hora a hora con esa base, marca `I ≥ 0,18`, convierte marcas en `[t − 24 h, t)` y repite hasta que el **conjunto de horas marcadas** no cambie. Se persiste siempre el `E` con el que se calculó la base guardada.
2. **Señal** (sobre registros fuera de `E` con B2 válido; mínimo 96).
3. **Componentes**: ventanas 6 h (fin cada 3 h) y 24 h (fin cada 6 h) alineadas a UTC, dentro del rango; se descartan las que solapan `E` o con cobertura < 70 %. Combinación con < 20 ventanas no se guarda.
4. **Exclusión**: marcas por I descendente (empate: t ascendente) hasta 40 % del histórico.
5. **Barrido horario**: horas alineadas a UTC `t ∈ [history_from + 24 h, history_to]` (cerrado en ambos extremos: la ventana que termina en `history_from + 24 h` es `[history_from, history_from + 24 h)` y está completa; con 90 días alineados son 24 × 90 − 23 = 2 137 horas), ventana `[t − 24 h, t)` sobre todos los samples (dos punteros). El spec original decía `(history_from + 24 h, …]`: era un error.

**Decisiones de implementación no explícitas en el spec**:
- `Confidence = NORMAL` exige las **7 combinaciones** (6 h: D, P, EX; 24 h: D, P, EX, C) con ≥ 60 ventanas. Si falta alguna ⇒ `LOW` (sin ella no se puede calcular I).
- "Sin estadísticas 24 h" = falta cualquiera de 24 h D, P, EX o C.
- `Δmin` cuando B2 tiene un solo valor distinto ⇒ `1` (resolución práctica de B2).
- Piso de Ex con señal degenerada ⇒ `Δmin`.
- Una nueva versión con `valid_from <=` al de la vigente se rechaza (`PERSISTING_BASELINE`): el versionado no es retroactivo.

## Eventos conocidos (exclusión forzada)

Además de lo que marca el barrido de I, hay intervalos que **nunca** deben entrar a la línea base porque corresponden a un evento conocido (fuga, intervención, reparación). Los entrega el port `KnownEventsProvider.GetKnownEvents(point, from, to)`.

- **Lista estática**: `infrastructure/knownevents.StaticProvider`, una lista en código por punto (`staticEvents`), con fechas en hora de Bogotá. Para el 11556 son los eventos de la tabla `PERIODOS` del notebook del analista: el periodo ambiguo, E1 (y extendido), E2, E3, E4 y E5. **No** incluye R1–R7 ni "Nuevo": son comportamiento normal.
- **Alertas**: `infrastructure/persistence/repositories.AlertEventsReader` lee `alerts` (tabla de Alert Management) con su propio modelo, solo lectura. Excluye **todas** las alertas del punto, cualquiera sea su clasificación (Confirmada, Falsa alerta o Sin clasificar), en `[started_at, ended_at)`; una alerta ACTIVA se excluye hasta el fin del rango consultado. Etiqueta del intervalo: `ALR-000123 (FUGA_CONFIRMADA_CAMPO)` (etiqueta, o clasificación, o `SIN_CLASIFICAR`). En cada recálculo (semanal, o diario con confianza `LOW`) se consulta de nuevo, así que las alertas nuevas entran solas.
- **Composición**: `internal/bootstrap/known_events.go` une ambas con `knownevents.CompositeProvider` (si una falla, falla todo). Si la tabla `alerts` no existe al arrancar, solo la lista estática. Alert Management no sabe que Baseline existe.

Reglas del cálculo (`RobustBaselineEstimator.EstimateExcluding`, `ExclusionPolicy.ApplyWithKnown`):

- El evento se recorta a `[history_from, history_to)` y se excluye **desde la primera pasada** y en todas: sus registros no entran a la señal y las ventanas que lo solapan no entran a los componentes.
- Se **une** a los periodos del barrido en `excluded_periods` (un periodo que solo viene de un evento tiene `peak_i = 0`).
- Sus horas **cuentan para el tope del 40 %**: las marcas automáticas solo disponen del resto.
- La convergencia sigue midiéndose solo sobre las horas marcadas por el barrido.
- Si algún evento cae en el histórico, la versión lleva el flag `KNOWN_EVENTS_EXCLUDED`.
- En el replay/backfill se aplican **con retrospectiva**: una versión simulada excluye la parte ya transcurrida de un evento aunque en su momento no se supiera que lo era.

Efecto a vigilar: al sacar los eventos la línea base queda más estrecha y el barrido marca más horas marginales (I entre 0,18 y 0,25), incluso en periodos de referencia. El total excluido sube y la estimación puede no converger (`BASELINE_REVIEW`).

## Flags

`SIGNAL_DEGENERATE`, `SIGNAL_LOW_VARIABILITY`, `NO_EXCLUSION_HISTORY`, `EXCLUSION_CAP_REACHED`, `BASELINE_REVIEW`, `KNOWN_EVENTS_EXCLUDED` (orden canónico). Triggers: `BOOTSTRAP`, `SCHEDULED`, `LOW_CONFIDENCE`, `MANUAL`.

## Política de recálculo (`RecalculationPolicy.IsDue`)

| Condición | Trigger |
|---|---|
| Sin versión | `BOOTSTRAP` |
| `LOW` o `NO_EXCLUSION_HISTORY` y `now − created_at ≥ 24 h` | `LOW_CONFIDENCE` |
| `now − created_at ≥ 168 h` | `SCHEDULED` |

`RecalcScheduler`: una ejecución al arrancar + tick horario en el minuto `BASELINE_TICK_MINUTE` (UTC), timeout 10 min por ejecución.

## Persistencia

Tablas creadas por **`AutoMigrate`** en `NewBaselineModule` (los tags del modelo generan también el índice parcial, el de vigencia, la FK y los CHECKs):

- `baseline_versions` — una fila por versión. `UNIQUE (measuring_point_id, version)`; `ux_baseline_versions_current` = índice único parcial `WHERE valid_to IS NULL` (**una sola vigente por punto**); `ix_baseline_versions_effective (measuring_point_id, valid_from DESC)`. `excluded_periods` JSONB `[{from, to, peak_i}]`, `flags` TEXT[]. Sin FK a `devices`.
- `baseline_component_stats` — PK `(baseline_version_id, window_hours, component)`, FK a `baseline_versions(id)`, MAD crudo (sin 1,4826 ni piso).

`SaveNewVersion` (una TX): `SELECT vigente FOR UPDATE` → `valid_to = ValidFrom` → `INSERT` versión `MAX(version)+1` + stats. SQLSTATE `23505` ⇒ `ErrBaselineConcurrentUpdate` (no-op).

`SaveHistory` (una TX, solo UC-B4): valida orden creciente de `valid_from` → verifica que el punto **no tenga versiones** (`ErrBaselineHistoryExists`) → `INSERT` de cada versión con `version = posición` y `valid_to = valid_from` de la siguiente (la última queda vigente) + stats. SQLSTATE `23505` (dos cargas concurrentes) ⇒ `ErrBaselineHistoryExists`. Si algo falla no queda ninguna versión.

Lecturas: `raw_measurements` (métricas 12/13/18, una consulta por punto, pivote en el mapper; NaN/±Inf ⇒ ausente) y `devices WHERE active = true` (la tabla de puntos de medición en código se llama `devices`).

## Errores (`domain/errors.go`)

| Error | Código | Cuándo |
|---|---|---|
| `ErrInvalidMeasuringPointID` | `INVALID_MEASURING_POINT_ID` | `pointID ≤ 0` |
| `ErrConsultingHistory` | `CONSULTING_MEASUREMENTS` | Falla `GetSamples` / `GetFirstSeenAt` / `ListActive…` |
| `ErrConsultingBaseline` | `CONSULTING_BASELINE` | Falla `GetLatest` |
| `ErrPersistingBaseline` | `PERSISTING_BASELINE` | Falla `SaveNewVersion` / `SaveHistory` |
| `ErrBaselineConcurrentUpdate` | `CONSULTING_BASELINE` | Otro proceso creó la vigente (no-op) |
| `ErrConsultingKnownEvents` | `CONSULTING_BASELINE` | Falla `GetKnownEvents`: no se calcula (una línea base contaminada es peor que ninguna) |
| `ErrBaselineHistoryExists` | `INVALID_INPUT` | `Backfill` sobre un punto que ya tiene versiones |

No son errores: `INSUFFICIENT_HISTORY`, `Status = NONE`, flags. El error crudo de GORM se loguea en el servicio y nunca sale del módulo.

## Configuración (entorno)

| Variable | Default |
|---|---|
| `BASELINE_METRIC_B1` / `_B2` / `_PLANITUD` | 12 / 13 / 18 |
| `BASELINE_HISTORY_DAYS` | 90 |
| `BASELINE_MIN_SIGNAL_RECORDS` | 96 |
| `BASELINE_MIN_COMPONENT_WINDOWS` | 20 (60 para `NORMAL` es fijo) |
| `BASELINE_EXCLUSION_I_CUTOFF` | 0.18 (tope 40 % fijo) |
| `BASELINE_MAX_PASSES` | 5 |
| `BASELINE_RECALC_INTERVAL` / `BASELINE_LOW_RECALC_INTERVAL` | `168h` / `24h` |
| `BASELINE_TICK_MINUTE` | 10 |

Un valor inválido se ignora con `Warn` y se usa el default.

## Aislamiento

`depguard` (`baseline-isolation`): no importa `modules/ingestion`, `modules/processing`, `modules/alert_management` ni `shared/eventbus`. Lee `raw_measurements`, `devices` y `alerts` con modelos propios. Solo `shared/{logger, errors, features}`. Hoy **no expone contrato público** a otros módulos: la fachada y el replayer se retiraron.

## Simulación de versiones (`ReplayService`)

`ReplayService.SimulateVersions(ctx, point, from, to)` devuelve las versiones que **habrían existido** en `[from, to)`: reloj virtual horario (horas en punto), `RecalculationPolicy` real sobre la última versión simulada y `RobustBaselineEstimator` con **solo** los datos de `[hora − 90 d, hora)`. Orden ascendente por `ValidFrom`, `ID = 0`, `Version` secuencial. Una sola lectura del histórico.

No recibe `BaselineRepository`: **no puede** escribir. Quien persiste la simulación es `BackfillService` (UC-B4), su único consumidor. Comparte con `Recalculate` la construcción de la versión (`newVersion`). Aproximación: producción recalcula en `BASELINE_TICK_MINUTE`; la simulación, en la hora en punto.

## Histórico retroactivo (UC-B4, `cmd/baseline -backfill`)

El versionado no es retroactivo: una versión creada hoy no aplica a horas pasadas. Para que un punto tenga la línea base que **habría tenido** en cada momento desde que hay datos, `BackfillService` persiste lo que simula `ReplayService.SimulateVersions`:

- **Rango**: desde `-from` (default: primer registro de B2 del punto, redondeado hacia arriba a la hora) hasta `-asof` (default: ahora).
- **Cadencia**: la de `RecalculationPolicy` real. Primera versión (`BOOTSTRAP`) en la primera hora con ≥ 96 registros de B2; mientras sea `LOW` / `NO_EXCLUSION_HISTORY`, una cada 24 h (`LOW_CONFIDENCE`); luego una cada 168 h (`SCHEDULED`).
- **Datos**: cada versión usa solo `[valid_from − 90 d, valid_from)`; nunca datos posteriores a su recálculo.
- **`created_at = valid_from`** (no la hora real de escritura): la política mide la edad con `created_at`, así el scheduler continúa el calendario desde la última versión retroactiva sin saltos. Una versión retroactiva **no se distingue** de una creada en su momento.
- **Precondición**: el punto no tiene versiones. Si tiene, hay que borrarlas antes (`baseline_component_stats` y luego `baseline_versions` del punto). `-dry-run` no exige la precondición: sirve para previsualizar.
- **Atomicidad**: una sola TX; o quedan todas las versiones o ninguna.

Flujo completo (primera ejecución y recálculos): `FLUJO_BASELINE.md`.

## Cableado y entrypoints

`bootstrap.NewBaselineModule(platform)` → `BaselineModule{Recalculator, Backfiller, NewScheduler}`.

| Entrypoint | Baseline |
|---|---|
| `cmd/platform` | Se compone después de `NewPlatform` y antes de Ingestion; su scheduler arranca primero y se espera antes de `Platform.Close` |
| `cmd/scheduler` | Vía `NewIngestionApplication()`; arranca su scheduler |
| `cmd/ingestion` | Se compone (migra) pero no arranca el scheduler |
| `cmd/baseline` | `baseline -point 11556 -asof 2026-07-01T00:00:00Z [-dry-run] [-json]` — validación con el analista (pasadas, θ_D, % excluido, ventanas, periodos excluidos). `baseline -point 11556 -backfill [-from …] [-asof …] [-dry-run] [-json]` — histórico retroactivo (UC-B4), una fila por versión |

## Tests

El módulo no tiene tests por ahora: se retiraron en el refactor de octubre de 2026.
