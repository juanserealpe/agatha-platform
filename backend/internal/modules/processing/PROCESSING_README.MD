# Modulo: Processing

**Responsabilidad**: cada hora y para cada punto activo, evaluar las ventanas de **24 h** y **6 h** que terminan en esa hora contra la línea base vigente y **persistir una fila por ventana y hora** en `feature_evaluations` (Cobertura, D, P, Ex, C, D_Pl, z, s(z), I) o el motivo por el que no se pudo calcular.

Processing **mide, no interpreta**: las horas seguidas, los cortes de I, las categorías y las alertas son del Rule Engine (decisión del analista, opción B). Todas las fórmulas vienen del kernel `shared/features`; Processing no reimplementa ninguna. Todo en **UTC** (`America/Bogota` solo en logs).

Además entrega **`cmd/replay`**: reproduce el pasado con el mismo código de producción, sin escribir en la BD, para calibrar con el analista.

## Estructura

```
processing/
├── domain/
│   ├── errors.go
│   ├── entities/        # BaselineSnapshot (vista propia de la línea base), FeatureEvaluation, enums
│   ├── services/        # Lógica pura (sin IO) + tests
│   │   ├── slot_planner.go       # qué horas evaluar
│   │   ├── window_slicer.go      # SliceWindow: [t − L, t) sin copiar
│   │   └── window_evaluator.go   # kernel + precedencia de estados → 2 filas por hora
│   └── repositories/    # Ports: MeasurementReader, EvaluationRepository
├── application/
│   ├── ports/           # BaselineProvider, Clock
│   └── services/        # EvaluationService (UC-P1 EvaluatePending, UC-P2 Reconcile), Config, comandos
└── infrastructure/
    ├── events/          # MeasurementsIngestedNotice + subscriber delgado
    ├── persistence/     # models (GORM), mappers, repositories (+ test integration)
    ├── replay/          # CSVEvaluationSink (EvaluationRepository en memoria → CSV)
    └── scheduling/      # HourlyScheduler (reconciliador)
```

## Entry Points

| Trigger | Acción | Componente |
|---|---|---|
| Evento `ingestion.measurements_ingested` (cada ~15 min) | Evalúa las horas listas del punto (máx. 24 por evento) | `MeasurementsIngestedSubscriber.Handle` → `EvaluationService.EvaluatePending` (`Trigger = EVENT`) |
| `HourlyScheduler` (al arrancar + minuto `PROCESSING_RECONCILE_MINUTE`, 35) | Para cada punto activo, completa las horas sin fila (máx. 72 por tick) | `EvaluationService.Reconcile` → `EvaluatePending` (`Trigger = RECONCILER`) |
| `cmd/replay` | Reproducción histórica a CSV, sin escribir en la BD | `bootstrap.RunReplay` → `EvaluatePending` (`Trigger = REPLAY`, nunca se persiste) |

## UC-P1 `EvaluatePending`

1. Valida `PointID > 0` (`ErrInvalidMeasuringPointID`, sin tocar la BD).
2. **Lock en proceso por punto**: el worker del bus y el reconciliador nunca evalúan el mismo punto a la vez.
3. `readyUntil`: el evento lo trae como `floorHour(watermark)`; si el comando no lo trae (reconciliador o evento sin watermark), `floorHour(MAX(timestamp) de B2)`. Nunca supera `floorHour(now)`.
4. `SlotPlanner`: horas en `(última evaluada, readyUntil]`; sin historial, las 72 que terminan en `readyUntil` (`PROCESSING_INITIAL_LOOKBACK_HOURS`). Tope por invocación. Vacío ⇒ fin (caso más común: 3 de cada 4 lotes).
5. **Una** lectura de samples `[primera − 24 h, última)` → `features.Normalize`.
6. Por cada hora en orden: `BaselineProvider.GetEffective(t)` → `SliceWindow` 24 h y 6 h → `WindowEvaluator` → 2 filas.
7. `SaveAll` en **una TX** con `ON CONFLICT DO NOTHING`. `inserted < filas` ⇒ otro proceso las insertó: `Info`, no error.
8. **Si falla la línea base o la lectura, aborta sin persistir nada**: las filas son inmutables y un fallo transitorio no debe quedar grabado como `NO_CALCULABLE`. El reconciliador reintenta.

## UC-P2 `Reconcile`

Para cada punto activo: `readyUntil = min(floorHour(now), max(floorHour(maxDataTs), floorHour(now − grace)))` (una hora está lista si llegó un dato en o después de ella, o si pasaron `PROCESSING_GRACE` = 30 min). Un error en un punto se registra y no detiene a los demás.

**Garantía para el Rule Engine**: toda hora `t ≤ floorHour(now − 30 min)` tiene fila tras el tick, salvo durante la recuperación de una caída de más de 72 h (se completa en los ticks siguientes, en orden y sin huecos).

`Warn` "posible evento perdido" cuando el reconciliador evalúa horas con `n_valid > 0` y el punto ya tenía historial (en la evaluación inicial no aplica).

## Ventanas y estados

- Hora evaluada `t` en punto UTC; ventana `[t − L, t)`: un registro exactamente en `t` pertenece a la siguiente. Timestamp de la muestra, nunca el de ingestión.
- Primero 24 h, luego 6 h: la de 6 h usa **C de 24 h** y la **estadística de C de 24 h** (`c_from_24h = true`).
- Un dato tardío nunca modifica una evaluación (si el comando trae `DataFrom` anterior a la última hora evaluada, se registra `Warn`).

Precedencia (gana el primero que se cumple). `NO_CALCULABLE` **no es un error** y siempre se persiste:

| # | Condición | Estado / motivo | Se guarda además |
|---|---|---|---|
| 1 | Sin línea base vigente | `NO_CALCULABLE` / `NO_SIGNAL_BASELINE` | n's, cobertura |
| 2 | Señal sin `MinElevatedB2` o flag `SIGNAL_DEGENERATE` | `NO_CALCULABLE` / `SIGNAL_DEGENERATE` | n's, cobertura |
| 3 | Cobertura < 70 % | `NO_CALCULABLE` / `LOW_COVERAGE` | n's, cobertura |
| 4 | Falta estadística de D, P, Ex (de la ventana) o C (de 24 h) | `NO_CALCULABLE` / `NO_COMPONENT_BASELINE` | D, P, Ex, C, D_Pl |
| 5 | C no calculable (< 40 registros con las tres señales en 24 h) | `NO_CALCULABLE` / `INSUFFICIENT_COHERENCE_DATA` | D, P, Ex, D_Pl |
| 6 | Resto | `CALCULATED` | todo, incluidos z, s(z), I |

Marcas (no cambian el estado): `low_confidence` (línea base `LOW`), `silent` (`t < first_seen + 7 d`; un dispositivo sin ningún registro también es silencioso), `MAD_FLOOR_APPLIED_<D|P|EX|C>`, `CONSTANT_SERIES_<B1|B2|PLANITUD>` (en 6 h se copian las de 24 h, porque C viene de ahí). Processing calcula también en modo silencioso y con confianza baja: el Rule Engine decide qué hacer con esas filas.

## Persistencia: `feature_evaluations`

Una fila por `(measuring_point_id, window_hours, evaluated_at)` — índice único `ux_feature_evaluations_point_window_hour` (idempotencia, "última hora evaluada" y lectura de las últimas ~72 filas del Rule Engine). `CHECK (window_hours IN (6, 24))`. Sin FK. La crea `AutoMigrate` en `NewProcessingModule`.

Columnas: `window_start`, `status`, `no_calc_reason`, `n_expected`/`n_received`/`n_valid`, `coverage`, `d`, `p_hours`, `ex`, `c`, `d_pl`, `c_from_24h`, `z_*`, `s_*`, `i`, `low_confidence`, `silent`, `flags` (TEXT[]), `baseline_version_id`, `baseline_version`, `algorithm_version` (`features.AlgorithmVersion`), `config_version` (`m=12,13,18`), `trigger`, `source_batch_seq`, `input_max_ts`, `created_at`. Con `algorithm_version` + `config_version` + `baseline_version_id` cualquier fila es reproducible.

Lee (solo lectura) `raw_measurements` (B1 = 12, B2 = 13, Planitud = 18) y `devices`, con su propio modelo de lectura.

## Contrato hacia Baseline

Port `BaselineProvider.GetEffective(ctx, pointID, at) (BaselineSnapshot, error)`; ausencia de línea base ⇒ `Available = false` (no es error). `BaselineSnapshot` usa los tipos del kernel (`features.Signal`, `map[int]features.ComponentStats`). El adapter `baselineProviderAdapter` (en `internal/bootstrap/processing.go`) traduce 1:1 `baseline.EffectiveBaseline` (punteros `P90Planitud`/`MinElevatedB2` ⇒ `Has*`). Sin caché.

## Contrato hacia el Rule Engine (no implementado acá)

Reglas de rachas decididas por el analista, solo sobre **I(24 h)**: `I ≥ corte` suma 1 en ese nivel; `I < corte` reinicia ese nivel; `NO_CALCULABLE` o sin fila tras la reconciliación congela todos; más de 12 `NO_CALCULABLE` seguidas reinicia todos. Niveles 0,18 / 0,35 / 0,70 anidados. Con confianza `LOW` la categoría se calcula igual, marcada. `confirmacion_planitud` (PD-P1) no se guarda: Processing guarda `D_Pl` y el Rule Engine aplicará el umbral.

## Herramienta de reproducción: `cmd/replay`

```
replay -point 11556 -from 2026-07-08T00:00:00Z -to 2026-10-03T00:00:00Z \
       -mode asof|fixed [-fixed-asof <RFC3339>] [-out replay_11556.csv]
```

| Modo | Línea base en cada hora `t` | Para qué |
|---|---|---|
| `asof` | La versión que habría estado vigente en `t` (`baseline.Replayer.SimulateVersions`: reloj virtual horario, `RecalculationPolicy` real, solo datos anteriores a cada recálculo) | Cómo se habría comportado el sistema |
| `fixed` | Una sola versión `baseline.Replayer.ComputeAt(-fixed-asof)` (default `-to`) | Reproducir el backtest del analista |

Es **UC-P1 con otros adaptadores**: lector real, `replayBaselineProvider` en memoria (bootstrap), `CSVEvaluationSink` y reloj virtual = `-to`. Evalúa las horas `[from, to]` con una sola lectura de samples `[from − 24 h, to)`.

- **No escribe**: no migra, no se suscribe al bus, no arranca schedulers y abre la conexión con `default_transaction_read_only=on`.
- CSV con las columnas de `feature_evaluations` + `mode` + `baseline_valid_from` (flags separados por `|`, NULL = vacío).
- Las horas seguidas las calcula el analista sobre el CSV (sección del Rule Engine).
- Esperado: `fixed` ≈ backtest (E1 I máx ≈ 0,99 y 64 h ≥ 0,70; E5 < 0,70). `asof` **no** coincide: el dataset empieza el 8 jul y las primeras horas serán `NO_SIGNAL_BASELINE`, luego confianza `LOW`.
- Aproximación: la simulación recalcula en la hora en punto (producción, en `BASELINE_TICK_MINUTE`).

## Errores

`domain/errors.go`, reutilizando códigos de `shared/errors`:

| Error | Código | Cuándo |
|---|---|---|
| `ErrInvalidMeasuringPointID` | `INVALID_MEASURING_POINT_ID` | `PointID ≤ 0` |
| `ErrConsultingMeasurements` | `CONSULTING_MEASUREMENTS` | Falla `GetSamples` / `GetLatestTimestamp` / `ListActiveMeasuringPoints` |
| `ErrConsultingBaseline` | `CONSULTING_BASELINE` | Falla el provider (infraestructura; la ausencia no es error) |
| `ErrConsultingEvaluations` | `CONSULTING_EVALUATIONS` | Falla `GetLastEvaluatedAt` |
| `ErrPersistingEvaluations` | `PERSISTING_EVALUATIONS` | Falla `SaveAll` |

En subscriber y scheduler no hay a quién responder: se registra `code` + mensaje y no se reintenta (el reconciliador es el reintento).

## Configuración

| Variable | Default | Notas |
|---|---|---|
| `PROCESSING_METRIC_B1` / `_B2` / `_PLANITUD` | 12 / 13 / 18 | Forman `config_version` |
| `PROCESSING_GRACE` | `30m` | Espera antes de evaluar una hora sin datos |
| `PROCESSING_MAX_SLOTS_EVENT` | 24 | Respeta el timeout de 30 s del bus |
| `PROCESSING_MAX_SLOTS_RECONCILER` | 72 | |
| `PROCESSING_INITIAL_LOOKBACK_HOURS` | 72 | Historia inicial para el Rule Engine |
| `PROCESSING_RECONCILE_MINUTE` | 35 | 0..59 |

Valor inválido ⇒ `Warn` y default.

## Reglas de Import (Aislamiento)

Processing solo importa `shared/{logger, errors, features}`. **No** importa `modules/ingestion`, `modules/baseline` ni `shared/eventbus` (`depguard` `processing-isolation` y `processing-no-ingestion`). La suscripción, la traducción del evento y el adapter hacia Baseline viven en `internal/bootstrap/processing.go`; `replayBaselineProvider` en `internal/bootstrap/replay.go`.

## Cableado (Composition Root)

`bootstrap.NewProcessingModule(platform, baselineModule)`: `AutoMigrate(feature_evaluations)` → repos → `baselineProviderAdapter{Facade}` → `EvaluationService` → suscripción `processing.measurements_ingested` → `ProcessingModule{Unsubscribe, NewScheduler}`.

| Entrypoint | Processing |
|---|---|
| `cmd/platform` | `Baseline → Processing → Ingestion → Dashboard`; schedulers Baseline, Processing, Ingestion |
| `cmd/scheduler` | Vía `NewIngestionApplication()`; arranca el reconciliador |
| `cmd/ingestion` | Compone Processing sin reconciliador: el evento evalúa las horas listas y `Platform.Close` drena |
| `cmd/api` | No lo usa |
| `cmd/replay` | Composición de solo lectura (`RunReplay`) |

- Debe registrarse **antes** de que Ingestion publique y vivir en el **mismo proceso** (bus en memoria).
- El evento (schema v1) trae `watermark`, `batch_seq` y `from`; `toMeasurementsIngestedNotice` los mapea a `MeasurementsIngestedNotice.Watermark`/`BatchSeq`/`From`. `watermark` es el `last_ingested_at` de Ingestion = MAX(timestamp) de las muestras del lote (no el fin del rango pedido), así que `watermark ≥ t` implica que ya existe un registro con timestamp `≥ t`. Si llegan en cero, quedan vacíos y Processing deriva la hora lista de la BD.

## Tests

`go test ./internal/modules/processing/... ./internal/bootstrap/...` (con `-race` para el lock). Integración contra PostgreSQL: `DATABASE_URL=... go test -tags integration ./internal/modules/processing/infrastructure/persistence/repositories/`.
