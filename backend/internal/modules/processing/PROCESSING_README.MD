# Modulo: Processing

**Responsabilidad**: cada hora y para cada punto activo, evaluar las ventanas de **24 h** y **6 h** que terminan en esa hora contra la línea base vigente en ella y **persistir una fila por ventana y hora** en `feature_evaluations` (Cobertura, D, P, Ex, C, D_Pl, z, s(z), I) o el motivo por el que no se pudo calcular.

Processing **mide, no interpreta**: las horas seguidas, los cortes de I, las categorías y las alertas son del Rule Engine. Todas las fórmulas vienen del kernel `shared/features`; Processing no reimplementa ninguna. Todo en **UTC** (`America/Bogota` solo en logs).

## Estructura

```
processing/
├── domain/
│   ├── errors.go
│   ├── entities/        # BaselineSnapshot (vista propia de la línea base), FeatureEvaluation, enums
│   ├── services/        # Lógica pura (sin IO)
│   │   ├── slot_planner.go       # qué horas evaluar
│   │   ├── window_slicer.go      # SliceWindow: [t − L, t) sin copiar
│   │   └── window_evaluator.go   # kernel + precedencia de estados → 2 filas por hora
│   └── repositories/    # Ports: MeasurementReader, EvaluationRepository
├── contract/            # Superficie pública (solo stdlib): TopicIndicatorEvaluated, IndicatorEvaluated
├── application/
│   ├── ports/           # BaselineProvider, Clock, EventPublisher
│   └── services/        # EvaluationService: UC-P1 EvaluatePending, UC-P2 Reconcile, UC-P3 Backfill
└── infrastructure/
    ├── events/          # BusEventPublisher / NoopEventPublisher (único lugar que conoce el bus)
    ├── persistence/     # models (GORM), mappers, repositories (incluye el lector de las tablas de Baseline)
    └── scheduling/      # HourlyScheduler (evaluación horaria)
```

> `infrastructure/replay/` (CSV) es del diseño anterior: **no está cableado** y queda pendiente de borrar. `MeasurementsIngestedNotice`/`Subscriber` se eliminaron el 2026-10-30.

## Entry Points

| Trigger | Acción | Componente |
|---|---|---|
| `HourlyScheduler` (al arrancar + minuto `PROCESSING_RECONCILE_MINUTE`, 35) | Para cada punto activo, evalúa las horas pendientes (máx. 72 por tick) | `EvaluationService.Reconcile` → `EvaluatePending` (`Trigger = RECONCILER`) |
| `cmd/processing -point` | Evalúa a demanda las horas pendientes de un punto | `EvaluationService.EvaluatePending` (`Trigger = MANUAL`) |
| `cmd/processing -point -backfill [-from] [-asof] [-dry-run] [-json] [-rebuild]` | Histórico de evaluaciones (`-rebuild`: reemplaza el rango) | `EvaluationService.Backfill` (`Trigger = BACKFILL`) |

El scheduler es el **único disparador automático**. No hay suscripción al bus: Ingestion no publica eventos.

## UC-P1 `EvaluatePending`

1. Valida `PointID > 0` (`ErrInvalidMeasuringPointID`, sin tocar la BD).
2. Lock en proceso por punto (el scheduler y un backfill nunca evalúan el mismo punto a la vez).
3. `readyUntil`: el que trae el comando; si no trae, `floorHour(MAX(timestamp) de B2)`. Nunca supera `floorHour(now)`.
4. **Primera hora evaluable** = primera hora en punto con una versión de línea base (`ceilHour(MIN(valid_from))`). **Si el punto no tiene ninguna versión, no escribe nada y espera** (lo avisa una sola vez): las filas son inmutables y no deben quedar grabadas como `NO_SIGNAL_BASELINE` solo porque Baseline aún no arrancó.
5. `SlotPlanner.PlanFrom`: horas en `(última evaluada, readyUntil]`; sin historial, las 72 que terminan en `readyUntil` (`PROCESSING_INITIAL_LOOKBACK_HOURS`), nunca antes de la primera evaluable. Tope por invocación. Vacío ⇒ fin.
6. **Una** lectura de samples `[primera − 24 h, última)` → `features.Normalize`; **una** lectura de las versiones de línea base vigentes en `[primera, última]`.
7. Por cada hora en orden: versión con `valid_from ≤ t < valid_to` → `SliceWindow` 24 h y 6 h → `WindowEvaluator` → 2 filas.
8. `SaveAll` en **una TX** con `ON CONFLICT DO NOTHING`.
9. **Si falla la línea base o la lectura, aborta sin persistir nada**. El siguiente tick reintenta.
10. **Post-commit**: publica `processing.indicator_evaluated` por cada fila (ver *Eventos Publicados*).

## Eventos Publicados

| Tópico | Evento (`contract/`) | Campos |
|---|---|---|
| `processing.indicator_evaluated` | `IndicatorEvaluated` | `measuring_point_id`, `window_hours` (24 \| 6), `evaluated_at` (fin exclusivo de la ventana), `status`, `no_calc_reason`, `i` (null si no se calculó), `low_confidence`, `trigger`, `occurred_at`, `schema_version` — tiempos en UTC |

- **Uno por ventana**: cada hora evaluada produce dos eventos (24 h y 6 h), también los `NO_CALCULABLE`.
- Lo publica **solo** `EvaluatePending` (scheduler `RECONCILER` y `cmd/processing` `MANUAL`) **después** de `SaveAll`, a través del port `EventPublisher`. Un error al publicar se registra con `Warn` y no falla la evaluación.
- **Backfill no publica**: el histórico son miles de filas y desbordaría la cola del bus (256 por suscripción); los consumidores leen `feature_evaluations`.
- **Solo si el lote se insertó completo**. `SaveAll` devuelve cuántas filas insertó, no cuáles (GORM 1.31 no mapea bien `RETURNING` con conflictos parciales), así que con un conflicto parcial (otro proceso ya insertó parte) no publica nada y lo avisa con `Warn`. Si se insertó 0, tampoco.
- Lleva `status` e `i` porque son el hecho que interesa (el Rule Engine los compara con sus reglas); la fila sigue siendo la fuente de verdad.
- Consumidores: Rule Engine (`rule_engine.indicator_evaluated`): interpreta las de 24 h `CALCULATED` con sus umbrales. Su backfill (`cmd/rule_engine -backfill`) lee directamente `feature_evaluations`: correrlo DESPUÉS de `processing -backfill`.

## UC-P2 `Reconcile`

Para cada punto activo: `readyUntil = min(floorHour(now), max(floorHour(maxDataTs), floorHour(now − grace)))` (una hora está lista si llegó un dato en o después de ella, o si pasaron `PROCESSING_GRACE` = 30 min). Un error en un punto se registra y no detiene a los demás.

No guarda estado en memoria: tras un corte retoma desde la última fila guardada, en orden y sin huecos. Una caída de más de 72 h se recupera en los ticks siguientes (72 horas por tick).

## UC-P3 `Backfill` (histórico)

Evalúa **todas** las horas de `[From, To]`, cada una con la versión de línea base que estaba vigente en ella, y las guarda en una sola TX.

- **Rango**: desde `-from` (default: primera hora con línea base) hasta `-asof` (default: última hora lista, la misma que usaría el scheduler). Nunca más allá de la última hora lista.
- **Solo completa lo que falta**: inserta con `ON CONFLICT DO NOTHING`, así que las filas existentes no se tocan. Se puede repetir y correr con el scheduler activo. No exige que el punto esté vacío.
- **Precondición práctica**: el histórico de Baseline (`cmd/baseline -backfill`) debe estar cargado **antes**. Sin él, el punto solo tiene la versión de hoy y no hay horas pasadas que evaluar.
- **`-rebuild`**: reemplaza en UNA TX (`EvaluationRepository.ReplaceRange`) las filas del punto (ambas ventanas) con `evaluated_at` en `[-from, última hora]` (sin `-from`: todas las anteriores) por las recalculadas. Es para después de un `baseline -backfill -rebuild`: las evaluaciones son inmutables en vivo, así que sin él las filas viejas quedan. Bloqueado con `AGATHA_ENV=production` (`ErrRebuildForbidden`, `Config.Environment`). No publica. Después: `rule_engine -backfill -rebuild` y `alert_management -backfill -rebuild`.
- `-dry-run` evalúa e imprime el resumen sin persistir; `-json` vuelca todas las filas.
- El resumen imprime filas por ventana y estado, y por día las horas calculadas y el mínimo, promedio y máximo de I(24 h). No clasifica: Processing solo calcula el valor.

## Ventanas y estados

- Hora evaluada `t` en punto UTC; ventana `[t − L, t)`: un registro exactamente en `t` pertenece a la siguiente. Timestamp de la muestra, nunca el de ingestión.
- Primero 24 h, luego 6 h: la de 6 h usa **C de 24 h** y la **estadística de C de 24 h** (`c_from_24h = true`).
- Un dato tardío nunca modifica una evaluación.

Precedencia (gana el primero que se cumple). `NO_CALCULABLE` **no es un error** y siempre se persiste:

| # | Condición | Estado / motivo | Se guarda además |
|---|---|---|---|
| 1 | Sin versión vigente en `t` | `NO_CALCULABLE` / `NO_SIGNAL_BASELINE` | n's, cobertura |
| 2 | Señal sin `MinElevatedB2` o flag `SIGNAL_DEGENERATE` | `NO_CALCULABLE` / `SIGNAL_DEGENERATE` | n's, cobertura |
| 3 | Cobertura < 70 % | `NO_CALCULABLE` / `LOW_COVERAGE` | n's, cobertura |
| 4 | Falta estadística de D, P, Ex (de la ventana) o C (de 24 h) | `NO_CALCULABLE` / `NO_COMPONENT_BASELINE` | D, P, Ex, C, D_Pl |
| 5 | C no calculable (< 40 registros con las tres señales en 24 h) | `NO_CALCULABLE` / `INSUFFICIENT_COHERENCE_DATA` | D, P, Ex, D_Pl |
| 6 | Resto | `CALCULATED` | todo, incluidos z, s(z), I |

El caso 1 ya no ocurre en el flujo normal (no se evalúan horas anteriores a la primera versión); queda como resguardo.

Marcas (no cambian el estado): `low_confidence` (línea base `LOW`), `silent` (`t < first_seen + 7 d`), `MAD_FLOOR_APPLIED_<D|P|EX|C>`, `CONSTANT_SERIES_<B1|B2|PLANITUD>` (en 6 h se copian las de 24 h, porque C viene de ahí).

## Persistencia: `feature_evaluations`

Una fila por `(measuring_point_id, window_hours, evaluated_at)` — índice único `ux_feature_evaluations_point_window_hour` (idempotencia y "última hora evaluada"). `CHECK (window_hours IN (6, 24))`. Sin FK. La crea `AutoMigrate` en `NewProcessingModule`.

Columnas: `window_start`, `status`, `no_calc_reason`, `n_expected`/`n_received`/`n_valid`, `coverage`, `d`, `p_hours`, `ex`, `c`, `d_pl`, `c_from_24h`, `z_*`, `s_*`, `i`, `low_confidence`, `silent`, `flags` (TEXT[]), `baseline_version_id`, `baseline_version`, `algorithm_version`, `config_version` (`m=12,13,18`), `trigger` (`RECONCILER`|`MANUAL`|`BACKFILL`), `source_batch_seq` (siempre NULL hoy), `input_max_ts`, `created_at`.

## Lectura de otros módulos

Processing **solo lee**, con sus propios modelos (no importa a ningún módulo):

| Tabla | Dueño | Para qué | Modelo de lectura |
|---|---|---|---|
| `raw_measurements` (B1 = 12, B2 = 13, Planitud = 18), `devices` | Ingestion | Samples, primer y último dato, puntos activos | `RawSampleRowModel` |
| `baseline_versions`, `baseline_component_stats` | Baseline | Versión vigente en cada hora | `BaselineVersionReadModel`, `BaselineComponentStatReadModel` |

Port `BaselineProvider` (`GetVersions(point, from, to)`, `GetFirstValidFrom(point)`), implementado por `repositories.BaselineReader`. Los modelos de lectura de Baseline **nunca** se pasan a `AutoMigrate`: el esquema es de Baseline. El acople es de esquema: si Baseline cambia esas columnas hay que actualizar `baseline_version_read_model.go`.

## Errores

| Error | Código | Cuándo |
|---|---|---|
| `ErrInvalidMeasuringPointID` | `INVALID_MEASURING_POINT_ID` | `PointID ≤ 0` |
| `ErrConsultingMeasurements` | `CONSULTING_MEASUREMENTS` | Falla `GetSamples` / `GetFirstTimestamp` / `GetLatestTimestamp` / `ListActiveMeasuringPoints` |
| `ErrConsultingBaseline` | `CONSULTING_BASELINE` | Falla la lectura de las tablas de Baseline (la ausencia de versiones no es error) |
| `ErrConsultingEvaluations` | `CONSULTING_EVALUATIONS` | Falla `GetLastEvaluatedAt` |
| `ErrPersistingEvaluations` | `PERSISTING_EVALUATIONS` | Falla `SaveAll` |

## Configuración

| Variable | Default | Notas |
|---|---|---|
| `PROCESSING_METRIC_B1` / `_B2` / `_PLANITUD` | 12 / 13 / 18 | Forman `config_version` |
| `PROCESSING_GRACE` | `30m` | Espera antes de evaluar una hora sin datos |
| `PROCESSING_MAX_SLOTS_RECONCILER` | 72 | Horas por punto y tick |
| `PROCESSING_INITIAL_LOOKBACK_HOURS` | 72 | Horas que evalúa el scheduler la primera vez (el resto, con `-backfill`) |
| `PROCESSING_RECONCILE_MINUTE` | 35 | 0..59 |

Valor inválido ⇒ `Warn` y default.

## Reglas de Import (Aislamiento)

Processing solo importa `shared/{logger, errors, features}`, y `shared/eventbus` **únicamente** en `infrastructure/events`. **No** importa `modules/ingestion`, `modules/baseline` ni `modules/rule_engine` (`depguard` `processing-isolation` y `processing-no-ingestion`); `domain/` y `application/` no conocen el bus (`processing-core-no-eventbus`) y `contract/` es solo stdlib (`processing-contract-stdlib-only`).

## Cableado (Composition Root)

`bootstrap.NewProcessingModule(platform)`: `AutoMigrate(feature_evaluations)` → lectores, repositorio y `BusEventPublisher(platform.EventBus)` → `EvaluationService` → `ProcessingModule{Evaluator, NewScheduler}`. No conoce a sus consumidores: la suscripción del Rule Engine vive en `bootstrap/rule_engine.go`.

| Entrypoint | Processing |
|---|---|
| `cmd/platform` | Módulo `processing` de `AGATHA_MODULES` (requiere `baseline`; **no** está en el default). Arranca el `HourlyScheduler` |
| `cmd/processing` | Evaluación a demanda y `-backfill`; sale al terminar. Publica en un bus sin suscriptores (no-op) |

**Orden para habilitarlo en un punto con historia**: `baseline -backfill` → `processing -backfill` → agregar `processing` a `AGATHA_MODULES`.

## Tests

Los de `domain/services` y `mappers` siguen vigentes. `application/services/indicator_event_test.go` cubre la publicación (commit → publica 2 por hora; rollback, conflicto total o parcial → no publica; fallo del bus no falla; Backfill no publica) y `infrastructure/events/bus_event_publisher_test.go` el adapter y la serialización JSON. El resto de `application/services` sigue pendiente.
