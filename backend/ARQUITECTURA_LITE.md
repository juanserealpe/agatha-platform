# Reglas Fijas y Restricciones (Guardrails)

1. **AISLAMIENTO MODULAR**: Prohibido importar paquetes internos entre módulos (`modules/dashboard` NUNCA importa `modules/ingestion/internal/...`). Comunicación solo vía interfaces públicas/facades en la raíz del módulo o eventos de `shared/eventbus`.
2. **CAPA SHARED OBLIGATORIA**: Prohibido `log.Println`, `json.NewEncoder` directo o `fmt.Errorf` genérico. Es obligatorio usar `shared/logger`, `shared/http` y `shared/errors`.
3. **ACCESO A BD COMPARTIDA**: Una sola BD para el monolito. `ingestion` gestiona la ESCRITURA de telemetría; `dashboard` realiza LECTURA (`SELECT`) de métricas, mediciones y evaluaciones (`feature_evaluations`) para visualización.
4. **ERRORES DE DOMINIO**: Envolver siempre con `shared/errors.AppError` antes de llegar al handler HTTP.
5. **CONTEXTO LOCAL Y VERSIÓN**: Todo agente DEBE consultar `backend/internal/modules/[modulo]/README.md` antes de modificar un módulo. Ignorar carpetas/archivos históricos `docs/` salvo instrucción explícita.
6. **EVENTOS ENTRE MÓDULOS**: El tipo del evento y su tópico viven en `modules/<publicador>/contract/` (solo stdlib). El consumidor NO importa al publicador (ni su `contract/`): define su propio tipo y `internal/bootstrap` (composition root, ÚNICO punto que conoce el bus y ambos módulos) traduce y suscribe. Se publica DESPUÉS del commit y un fallo al publicar nunca falla la operación. Reglas completas: `internal/shared/eventbus/README.md`. Reforzado con `depguard` en `backend/.golangci.yml`.

---

# Capa Shared

## Logger (`shared/logger`)

Logger estructurado con nivel y módulo. Timezone: `America/Bogota`.

```go
l := logger.New("ingestion", "INFO")
l.Info("mediciones guardadas", "cantidad", 42)
```

`Debug` solo se imprime con un logger creado con nivel `DEBUG` (detalle de diagnóstico; `Info`/`Warn`/`Error` se imprimen siempre).

## HTTP Wrappers (`shared/http`)

- `WriteJSON(w, status, data)` — respuesta JSON estandarizada.
- `HandleError(w, err)` — mapea `*AppError` a HTTP status automáticamente.

```go
sharedhttp.HandleError(w, appErrors.ErrNotFound) // → 404
sharedhttp.WriteJSON(w, http.StatusOK, payload)
```

## Event Bus (`shared/eventbus`)

Bus en memoria, no bloqueante, cola acotada + worker FIFO por suscripción (overflow `DropOldest` por defecto). Best-effort: los eventos son pistas, la BD es la fuente de verdad. Se crea y se cierra (`Close` drena) solo en `internal/bootstrap`.

```go
eventbus.PublishTyped(ctx, bus, contract.TopicIndicatorEvaluated, event)
eventbus.SubscribeTyped(bus, topic, handler, log, eventbus.WithName("rule_engine.indicator_evaluated"))
```

**Catálogo de eventos**:

| Tópico | Publicador | Consumidores |
|---|---|---|
| `processing.indicator_evaluated` | Processing (`EvaluatePending`, post-commit; uno por ventana 24 h/6 h; no en backfill) | Rule Engine (`EvaluateIndicator`: interpreta 24 h `CALCULATED`, reenvía 24 h `NO_CALCULABLE`) |
| `rule_engine.level_evaluated` (v3, aditiva sobre v2) | Rule Engine (post-commit; **cada** hora de 24 h: `CALCULATED`, incluida NORMAL, y `NO_CALCULABLE` sin fila; no en backfill, duplicados ni llegadas tardías) | Alert Management (suscripción `alert_management.level_evaluated` → `ProcessingService.Process`; duplicados por `(punto, evento, evaluated_at)` se ignoran) |
| `alert_management.alert_notification` (v1) | Alert Management (post-commit; **cada** mensaje `NOTIFICACION` nuevo de una hora `LIVE` o `CATCH_UP`; no en backfill ni reprocesos; `late` marca horas con más de `late_notification_max_hours` de atraso) | Ninguno todavía: lo consumirá el módulo de notificaciones (web, push), idempotente por `message_id`. Sin suscriptores es un no-op |

## Errors (`shared/errors`)

`AppError{Code, Message}` + códigos tipados (`ErrorCode`). Errores de dominio se definen en `domain/errors.go` de cada módulo reutilizando los códigos compartidos.

```go
var ErrCustom = &errors.AppError{Code: errors.CodeNotFound, Message: "recurso no encontrado"}
```

**Códigos disponibles**: `NOT_FOUND`, `INVALID_INPUT`, `CONFLICT` (409: escritura concurrente, el cliente reintenta), `FUTURE_DATE`, `ALREADY_INGESTED`, `BEFORE_INITIAL_DATE`, `INVALID_MEASURING_POINT_ID`, `CONSULTING_LAST_STATE`, `GETTING_INGESTED_DATA`, `CONSULTING_DEVICES`, `CONSULTING_MEASUREMENTS`, `CONSULTING_BASELINE`, `PERSISTING_BASELINE`, `CONSULTING_EVALUATIONS`, `PERSISTING_EVALUATIONS`, `CONSULTING_THRESHOLDS`, `PERSISTING_THRESHOLDS`, `CONSULTING_RULE_EVALUATIONS`, `PERSISTING_RULE_EVALUATIONS`.

## Kernel de cálculo (`shared/features`)

Fórmulas de línea base e índice I compartidas por Baseline y Processing: `Normalize`, `Quantile` (tipo 7), `Median`, `MAD`, `Spearman`, `ComputeComponents`, `MADFloor`, `Score`. Solo stdlib (`depguard` `features-stdlib-only`), sin IO ni estado, versionado con `AlgorithmVersion` (persistido en cada versión de línea base). Detalle: `internal/shared/features/README.md`.

---

# Modulo: Ingestion

**Responsabilidad**: Recibir telemetría y persistirla con control transaccional (Unit of Work). Vía **principal**: callback HTTP de Dataflow Labs (`POST /ingestion/callback`, módulo `callback` de `AGATHA_MODULES`). Vía de **contingencia**: consulta programada a DataOS vía `IngestionScheduler` (módulo `dataos`). Se usa una sola a la vez: el índice único de `raw_measurements` descarta mediciones repetidas, pero cada vía crea sus propios lotes en `ingestion_state_models`. Detalle en `modules/ingestion/INGESTION_README.md`.

**Arquitectura**: Hexagonal — `application/ports` define interfaces (`DataProvider`, `IngestionUnitOfWork`), `domain/` contiene entidades y repositorios, `infrastructure/` implementa persistencia, cliente externo DataOS y el handler HTTP del callback. No publica eventos: los consumidores (Baseline, Processing) leen `raw_measurements` por su propio scheduler.

## Endpoints / Entry Points

| Trigger | Accion | Servicio |
|---------|--------|----------|
| `POST /ingestion/callback` (Bearer `AGATHA_CALLBACK_TOKEN`) | Valida el registro de Dataflow Labs, mapea sus variables a `metric_id` (`CallbackMapper`; las sin mapeo se avisan con WARN) y persiste en una sola TX el recibo en `ingestion_state_models` (idempotente por `record_id`) y las mediciones en `raw_measurements` | `CallbackHandler.ReceiveRecord` → `CallbackService.Receive` |
| `IngestionScheduler.Start()` | Ingesta periódica por intervalo configurable (contingencia) | `IngestService.IngestDataOs(ctx, measuringPointID, start, end)` |

**Callback**: 200 `stored`/`duplicate`; 400 (JSON o campos inválidos), 401, 404 (`mp_id` no registrado), 405, 413 (> 1 MiB), 415, 500 (fallo de BD ⇒ Dataflow Labs reintenta). Timestamps sin zona **o con sufijo `Z`** se interpretan en `America/Bogota` (Dataflow Labs envía la hora local de Colombia marcada como UTC: la misma lectura llega `11:45:41Z` por callback y `16:45:41Z` por DataOS); un offset explícito (`-05:00`) se respeta; `readings` se conserva como literal JSON. Deduplicación: índice único `ux_ingestion_logs_record_id` + `ON CONFLICT (record_id) DO NOTHING` (seguro ante concurrencia).

## DTOs Clave

| DTO | Campos |
|-----|--------|
| `DataRecordCallback` | `event`, `record_id`, `mp_id`, `serial_id`, `timestamp`, `records []{device_id, timestamp, name, readings}` |
| `CallbackResponse` | `status` (`stored`\|`duplicate`), `record_id`, `received_records` |

## Entidades de Dominio

- `Device` — `MeasuringPointID`, `Name`, `Active`
- `RawMeasurement` — `Timestamp`, `MeasuringPointID`, `MetricID`, `Value`, `Source`, `IngestionBatchID` (lote que la trajo)
- `IngestionState` — `MeasuringPointID`, `Source`, `LastIngestedAt`, `ReceivedRecords`, `RecordID` (solo callback)
- `CallbackRecord` / `CallbackReading` — registro del callback validado (timestamps parseados + literales originales)
- `SaveBatchResult` — `Inserted` (filas realmente insertadas por `MeasurementRepository.SaveBatch`)
- `SaveStateResult` — `BatchSeq`, `LastIngestedAt` (devuelto por `IngestionStateRepository.Save`)

## Flujo de Ingesta

`Scheduler` → `IngestService.IngestDataOs` → validar rango → buscar último estado → `DataProvider.GetMeasurements` → `UnitOfWork.Execute` (en TX: primero el estado, cuyo `id` es el `ingestion_batch_id` de cada medición, y después el lote).

`last_ingested_at` = **MAX(timestamp) de las muestras del lote** (`getLatestMeasurementTimestamp` en `IngestService`), NO el fin del rango pedido a DataOS. Si falla el `Save` del estado o el lote, la TX completa se revierte (no quedan mediciones sin su fila de estado) y se devuelve `ErrPersistingIngestionState` (`PERSISTING_INGESTION_STATE`).

Ingestion **no publica eventos** (el evento `ingestion.measurements_ingested` se eliminó el 2026-10-30: no tenía suscriptores).

---

# Modulo: Dashboard

**Responsabilidad**: Exponer datos de dispositivos y mediciones ya ingeridos para consumo del frontend/dashboard. Solo LECTURA.

**Arquitectura**: Hexagonal — handlers HTTP en `infrastructure/http/`, servicio en `application/services/`, repositorios como interfaces en `domain/repositories/`.

## Endpoints

| Metodo | Handler | Servicio | Descripcion |
|--------|---------|----------|-------------|
| `GET` | `DeviceHandler.GetAllDevices` | `DashboardService.GetAllDevices` | Lista todos los dispositivos con última conexión y batería |
| `GET` | `MeasurementHandler.GetMeasurements` | `DashboardService.GetMeasurements` | Mediciones por `deviceId`, `startDate`, `endDate` (query params RFC3339) |
| `GET` | `SeriesHandler.GetVariables` | `SeriesService.GetVariables` | Catálogo de variables graficables |
| `GET` | `SeriesHandler.GetSeries` | `SeriesService.GetSeries` | Series por `deviceId`, `startDate`, `endDate` y `variables` opcional (hoy `b1`, `b2`, `densidad`, `indicador`, `temperatura`, `humedad`, `deriva`, `pitch`, `roll`, `aceleracion`, `voltaje`, `corriente`). Lee `raw_measurements` y `feature_evaluations`. Agregar una variable es una línea en `domain/entities/series_catalog.go` |

## DTOs Clave

| DTO | Campos |
|-----|--------|
| `DeviceResponse` | `id int64`, `name string`, `lastConnection *time.Time`, `lastBatteryLevel *float64` |
| `MeasurementsResponse` | `deviceId int64`, `startDate`, `endDate`, `measurements []MetricMeasurementsResponse` |
| `MetricMeasurementsResponse` | `metricId uint64`, `metricName string`, `unit string`, `data []MeasurementPointResponse` |
| `MeasurementPointResponse` | `timestamp time.Time`, `value float64` |
| `SeriesResponse` | `deviceId int64`, `startDate`, `endDate`, `series []VariableSeriesResponse` |
| `VariableSeriesResponse` | `key string`, `name string`, `unit string`, `source string` (`measurement`\|`evaluation`), `data []MeasurementPointResponse` |

## Validaciones

- `DashboardValidator.ValidateDateRange` — `startDate` debe ser estrictamente anterior a `endDate` (rechaza igualdad).
- `DashboardValidator.ValidateNotFuture(date, err)` — genérica; se aplica a `startDate` y `endDate` por separado, ninguna puede ser futura.
- `deviceId` validado en el handler como entero positivo obligatorio; fechas validadas como RFC3339 obligatorias.
- Existencia del dispositivo verificada antes de consultar mediciones (404 si no existe).
- Rango de consulta `[startDate, endDate)`. Errores de repositorio siempre envueltos en `domain/errors.go` del módulo (`ErrConsultingDevices`, `ErrConsultingMeasurements`) antes de llegar al handler.





# Modulo: Baseline (Device Context & Baseline)

**Responsabilidad**: mantener por dispositivo, versionada en el tiempo, la línea base estadística (señal: `mediana_B2`, `θ_D = P90_B2`, `P90_Planitud`; componentes: mediana y MAD de D, P, Ex en 6 h/24 h y C en 24 h) con estimación iterativa que excluye periodos anómalos (`I(24 h) ≥ 0,18`, tope 40 %, máx. 5 pasadas). Detalle en `modules/baseline/BASELINE_README.md`.

**Eventos conocidos**: los intervalos de fugas/intervenciones conocidas se excluyen siempre del cálculo (además de lo que marca el barrido) y la versión lleva el flag `KNOWN_EVENTS_EXCLUDED`. Los entrega el port `KnownEventsProvider`, que `internal/bootstrap/baseline_alert_events.go` arma con `AlertEventsReader` (modelo de lectura propio de `alerts`: **todas** las alertas del punto, cualquiera sea su clasificación; solo `started_at`/`ended_at`, las activas hasta el fin del rango). El dominio amplía cada alerta con un margen de 2 h antes y después (`BASELINE_KNOWN_EVENT_MARGIN`). Sin tabla `alerts`, no se excluye nada. Alert Management no sabe que Baseline existe.

**Contrato público**: ninguno por ahora. La fachada (`Facade.GetEffectiveBaseline`) y el `Replayer` se retiraron en el refactor de octubre de 2026; el módulo solo se usa desde su scheduler y `cmd/baseline`.

| Trigger | Accion | Componente |
|---|---|---|
| `RecalcScheduler` (al arrancar + cada hora en `BASELINE_TICK_MINUTE`) | Recalcula los puntos activos a los que les toca (sin versión ⇒ `BOOTSTRAP`; `LOW`/`NO_EXCLUSION_HISTORY` cada 24 h; resto cada 168 h) | `RecalculationService.RecalculateDue` → `Recalculate` |
| `cmd/baseline -point -asof [-dry-run] [-json]` | Recálculo manual / validación con el analista | `RecalculationService.Recalculate` |
| `cmd/baseline -point -backfill [-from] [-asof] [-dry-run] [-json]` | Histórico retroactivo: persiste, en una TX, las versiones que habrían existido desde el primer registro (política real, `created_at = valid_from`). Solo en un punto sin versiones | `BackfillService.Backfill` → `ReplayService.SimulateVersions` + `SaveHistory` |

**Aislamiento**: solo LEE `raw_measurements` y `devices` (de Ingestion) y `alerts` (de Alert Management), con modelos de lectura propios y sin FK, y ESCRIBE `baseline_versions` / `baseline_component_stats`. No importa `ingestion`, `processing`, `alert_management` ni `shared/eventbus` (`depguard` `baseline-isolation`). No publica ni consume eventos.

**Errores**: `ErrInvalidMeasuringPointID`, `ErrConsultingHistory` (`CONSULTING_MEASUREMENTS`), `ErrConsultingBaseline`, `ErrPersistingBaseline`, `ErrBaselineConcurrentUpdate` (no-op), `ErrConsultingKnownEvents` (`CONSULTING_BASELINE`: sin la lista de eventos no se calcula), `ErrBaselineHistoryExists` (`INVALID_INPUT`: backfill sobre un punto con versiones). `INSUFFICIENT_HISTORY` y los flags no son errores.

---

# Modulo: Processing

**Responsabilidad**: cada hora y para cada punto activo, evaluar las ventanas de 24 h y 6 h que terminan en esa hora contra la línea base vigente en ella y persistir **una fila por (punto, ventana, hora)** en `feature_evaluations` (Cobertura, D, P, Ex, C, D_Pl, z, s(z), I) o el motivo `NO_CALCULABLE`. **Mide, no interpreta**: rachas, cortes, categorías y alertas son del Rule Engine. Todo cálculo pasa por `shared/features`. Detalle en `modules/processing/PROCESSING_README.MD`.

| Trigger | Accion | Componente |
|---|---|---|
| `HourlyScheduler` (al arrancar + minuto 35) | Evalúa las horas pendientes de cada punto activo (máx. 72 por tick; grace 30 min). Único disparador automático | `EvaluationService.Reconcile` → `EvaluatePending` |
| `cmd/processing -point` | Evalúa a demanda las horas pendientes de un punto | `EvaluationService.EvaluatePending` |
| `cmd/processing -point -backfill [-from] [-asof] [-dry-run] [-json]` | Histórico: evalúa todas las horas del rango con la versión de línea base vigente en cada una; solo inserta las que faltan | `EvaluationService.Backfill` |

**Línea base**: Processing la lee directo de `baseline_versions` / `baseline_component_stats` con un modelo de solo lectura propio (port `BaselineProvider`, adapter `repositories.BaselineReader`). Baseline no expone nada: el acople es de esquema. Un punto sin ninguna versión **no se evalúa** (se espera a la primera), y nunca se evalúan horas anteriores a la primera versión.

**Garantías**: idempotente (índice único + `ON CONFLICT DO NOTHING`); una TX por lote (todas las horas o ninguna); un fallo de lectura **no** persiste filas `NO_CALCULABLE` falsas (aborta y el siguiente tick reintenta); sin estado en memoria: tras una caída retoma desde la última fila guardada, en orden y sin huecos.

**Errores**: `ErrInvalidMeasuringPointID`, `ErrConsultingMeasurements`, `ErrConsultingBaseline`, `ErrConsultingEvaluations` (`CONSULTING_EVALUATIONS`), `ErrPersistingEvaluations` (`PERSISTING_EVALUATIONS`). `NO_CALCULABLE`, ausencia de línea base y conflicto de unicidad no son errores.

**Eventos**: publica `processing.indicator_evaluated` (`contract.IndicatorEvaluated`: `measuring_point_id`, `window_hours`, `evaluated_at`, `status`, `no_calc_reason`, `i`, `low_confidence`, `trigger`, `occurred_at`, `schema_version`) por cada fila persistida por `EvaluatePending`, a través del port `EventPublisher`. Solo si el lote se insertó completo (con conflicto parcial no se sabe qué filas son propias ⇒ `Warn` y no publica). `Backfill` no publica. No consume eventos.

**Aislamiento**: solo importa `shared/{logger, errors, features}` y `shared/eventbus` únicamente en `infrastructure/events`; no importa `ingestion`, `baseline` ni `rule_engine` (`depguard` `processing-isolation`, `processing-core-no-eventbus`, `processing-contract-stdlib-only`).

**Inicialización**: `bootstrap.NewProcessingModule(platform)`. Para un punto con historia: `baseline -backfill` → `processing -backfill` → habilitar `processing` en `AGATHA_MODULES`.

---

# Modulo: Rule Engine

**Responsabilidad**: interpretar cada hora calculada de I(24 h) con **umbrales versionados por punto** (Amarillo/Naranja/Rojo) y reglas de **persistencia** (sube un nivel por racha) e **histéresis** (salida bajo `amarillo − exit_margin` durante `exit_hours`), guardar una fila por hora en `rule_evaluations` y publicar `rule_engine.level_evaluated` (v3) por **cada** hora de 24 h con `raw_level`, `computed_level` (max(raw, persistencia), sin histéresis; NARANJA sin confirmar por planitud baja a AMARILLO), `episode_level`, `episode_event` (`OPENED|CLEARED|NONE`), `reasons[]`, umbrales y salidas por nivel, y (v3) `runs`, `escalation`, `below_run`, `d_pl` y `persistence_mode`. Persistencia `STRICT` (racha seguida) o `WINDOW` (horas en ventana móvil) por versión; confirmación opcional de NARANJA con `d_pl` (leído de `feature_evaluations`). Las horas `NO_CALCULABLE` no tienen fila (un hueco pausa las rachas, no las reinicia) y se reenvían con `status = NO_CALCULABLE`. El Rule Engine es dueño del **nivel del indicador**; la alerta (estabilización, atención, clasificación) es de Alert Management. Detalle (algoritmo, escenario, contratos) en `modules/rule_engine/RULE_ENGINE_README.MD`.

| Trigger | Accion | Componente |
|---|---|---|
| Evento `processing.indicator_evaluated` (suscripción `rule_engine.indicator_evaluated`) | 24 h `CALCULATED`: versión vigente en `evaluated_at` + últimas K filas → lógica pura → `INSERT … ON CONFLICT DO NOTHING` → post-commit publica `level_evaluated`. 24 h `NO_CALCULABLE`: sin fila, se reenvía. Duplicado ⇒ nada; llegada tardía ⇒ guarda sin publicar | `IndicatorEvaluatedSubscriber` → `EvaluationService.EvaluateIndicator` |
| Catch-up horario (al arrancar + minuto `RULE_ENGINE_CATCHUP_MINUTE`, 50) | Red de seguridad de la suscripción: interpreta **publicando**, con el MISMO caso de uso, las horas `CALCULATED` de 24 h de `feature_evaluations` posteriores a la última fila de cada punto (lote de Processing insertado en parte, evento perdido, error al consumir). Punto sin filas ⇒ se saltea (espera a `-backfill`) | `CatchUpScheduler` → `CatchUpService.CatchUp` |
| `cmd/rule_engine -backfill [-point] [-from] [-to] [-rebuild]` | Histórico desde `feature_evaluations` con el MISMO caso de uso sin publicar; idempotente; `-rebuild` bloqueado con `AGATHA_ENV=production` | `BackfillService.Backfill` |
| `GET/POST /api/rule-engine/points/{id}/thresholds`, `GET …/history`, `GET …/timeline?from=&to=`, `POST /api/rule-engine/thresholds/bulk` | Consulta y creación de versiones (patch; 400 validación, 409 concurrencia). **Sin auth todavía**: `created_by` en el cuerpo (temporal) | `ThresholdHandler` → `ThresholdService` |
| `POST /api/rule-engine/points/{id}/thresholds/simulate` | Histórico con parámetros propuestos sobre un sandbox en memoria; no escribe ni publica | `ThresholdHandler` → `SimulationService` |

**Versionado**: solo se agregan versiones; la única modificación es cerrar la abierta (`valid_to`) en la misma TX que crea la nueva (`valid_from >= now`). `EXCLUDE USING gist` impide solapamientos. Excepción documentada: el seed de la v1 (`valid_from` = primera medición del punto), que corre una sola vez con la tabla vacía. Punto nuevo ⇒ v1 con `valid_from = now` en el primer uso.

**Aislamiento**: define su propio `IndicatorEvaluatedNotice`; no importa `processing` (ni su `contract/`), `ingestion`, `baseline` ni `shared/eventbus` (`depguard` `rule-engine-isolation`); `contract/` solo stdlib (`rule-engine-contract-stdlib-only`). Lee `feature_evaluations` con un modelo propio (port `IndicatorSource`). Publica por el port `LevelEventPublisher`; la traducción, la suscripción y el adaptador al bus viven en `internal/bootstrap/rule_engine.go`.

**Errores**: validación `INVALID_INPUT` (orden, rangos, planitud, ventana, cuerpo/patch, autoría, fecha pasada, rango de simulación, rebuild en production), `ErrThresholdConflict` (`CONFLICT`), `ErrThresholdsNotFound`, `CONSULTING_THRESHOLDS`, `PERSISTING_THRESHOLDS`, `CONSULTING_RULE_EVALUATIONS`, `PERSISTING_RULE_EVALUATIONS`. Hora ignorada/duplicada/tardía/sin umbrales no es error.

**Inicialización**: `baseline -backfill` → `processing -backfill` → `rule_engine -backfill` (con `rule_threshold_versions` vacía, el seed crea la v1 de cada punto con la plantilla —amarillo 0,16, salida 0,14— desde su primera medición) → habilitar `rule_engine` en `AGATHA_MODULES` (el catch-up completa, publicando, las horas entre el backfill y el arranque).

---

# Modulo: Alert Management

**Responsabilidad**: convertir cada hora de `rule_engine.level_evaluated` (v3) en el ciclo de la alerta para el operador (web) y el usuario de campo (app): una alerta ACTIVA por (punto, evento) con ID `ALR-000123`, nivel actual y máximo, colores (tramos) con sus mensajes, reactivación dentro de 24 h si está Sin clasificar, bajada de un nivel tras 6 h (el rojo sin clasificar no baja), qué mensajes son notificaciones (los publica como `alert_management.alert_notification` para el módulo de notificaciones: **no entrega avisos**, sin push ni FCM), y la gestión del operador (clasificar → etiquetar, observaciones; inmutable, idempotente por `client_record_id`, conflictos RN-18). **No recalcula niveles ni cambia umbrales** (son del Rule Engine). Especificación: `backend/AGATHA_Modulo_Alert_Management_v3.3.docx` (sin fotos). Detalle en `modules/alert_management/README.MD`.

| Trigger | Accion | Componente |
|---|---|---|
| Evento `rule_engine.level_evaluated` (suscripción `alert_management.level_evaluated`) | Bajo lock por punto: política vigente + estado + alerta activa → `Decide` (puro: transiciones + reglas de mensaje en orden) → UNA TX (alerta, colores, mensajes, estado) → después del commit publica cada mensaje `NOTIFICACION` nuevo (`alert_management.alert_notification`). Hora ≤ última procesada ⇒ nada. Un salto de `evaluated_at` se procesa como horas `NO_CALCULABLE` | `LevelEvaluatedSubscriber` → `ProcessingService.Process` (`LIVE`) |
| Catch-up (al arrancar + minuto `ALERT_MANAGEMENT_CATCHUP_MINUTE`, 55) | Horas de `rule_evaluations` posteriores a la última procesada, publicando avisos (`CATCH_UP`; con más de `late_notification_max_hours` de atraso salen con `late = true`). Punto sin estado ⇒ se saltea (espera a `-backfill`) | `CatchUpScheduler` → `ReplayService.CatchUp` |
| `cmd/alert_management -backfill [-point] [-from] [-to] [-rebuild]` | Histórico desde `rule_evaluations` con el MISMO caso de uso, sin publicar avisos (`BACKFILL`); idempotente; `-rebuild` borra todo lo del punto (bloqueado en production) | `ReplayService.Backfill` |
| `/api/alerts/*` | Historial (filtros, 10 por página), `export.xlsx` (≤ 10.000), `pending`, `active`, `{id}` (o `ALR-…`), `devices/{id}/alerts`, `POST {id}/classification|label|observations`, `policies` (+ `history`). **Sin auth todavía**: `origin` (+ `device_identifier` desde la app) y `created_by` en el cuerpo | `AlertHandler` → `QueryService`, `ManagementService`, `PolicyService` |

**Inmutabilidad**: clasificación y etiqueta se escriben con `UPDATE … WHERE classification IS NULL` (o `label IS NULL`); el trigger `trg_alerts_management_immutable` rechaza cualquier cambio posterior y todo `DELETE` (salvo `SET LOCAL agatha.alerts_rebuild = 'on'` del `-rebuild`). El procesamiento horario solo escribe sus columnas (`ProcessingColumns`).

**Aislamiento**: define su propio `LevelNotice`; no importa `rule_engine` (ni su `contract/`), `processing`, `ingestion`, `baseline` ni `shared/eventbus` (`depguard` `alert-management-isolation`). Lee `rule_evaluations` + `rule_threshold_versions` y `devices` con modelos propios. La traducción del evento, la suscripción y el adaptador `NotificationPublisher` → bus viven en `internal/bootstrap/alert_management.go`. No expone fachada; no conoce a Baseline (Baseline lee `alerts` por su cuenta).

**Inicialización**: `rule_engine -backfill` → `alert_management -backfill` → habilitar `alert_management` en `AGATHA_MODULES` (requiere `rule_engine`).

---

# Composition Root y Despliegue

`internal/bootstrap` arma cada módulo sobre una `Platform` compartida (`NewPlatform`: `.env` + `DATABASE_URL` + bus en memoria). Cada módulo tiene su constructor y ninguno conoce a los demás:

| Constructor | Módulo | Notas |
|---|---|---|
| `NewPlatform()` | Shared | BD única + `InMemoryBus`. `Platform.Close(ctx)` drena el bus. |
| `NewBaselineModule(platform)` | Baseline | AutoMigrate (tablas + índice parcial vía tags) + config `BASELINE_*`. Expone `Recalculator`, `Backfiller` y `NewScheduler()`. |
| `NewIngestionModule(platform)` | Ingestion (DataOS) | AutoMigrate + DataOS + publisher. `.NewScheduler()` crea el scheduler (punto 11556, cada 15 min, +50 s). |
| `NewCallbackModule(platform)` | Ingestion (callback) | AutoMigrate (helper compartido `migrateIngestionSchema`) + `CallbackService` + handler. Lee `AGATHA_CALLBACK_TOKEN` (vacía ⇒ arranca y responde 401 a todo). No requiere credenciales de DataOS. `.RegisterRoutes(mux)` registra `/ingestion/callback`. |
| `NewProcessingModule(platform)` | Processing | AutoMigrate (`feature_evaluations` + índice único vía tags) + config `PROCESSING_*` + `BusEventPublisher`. Lee las tablas de Baseline e Ingestion con modelos propios. Expone `Evaluator` y `NewScheduler()`. |
| `NewRuleEngineModule(platform)` | Rule Engine | `persistence.Migrate` (AutoMigrate + `btree_gist` + `EXCLUDE` idempotente) + seed de la v1 + repos + `busLevelEventPublisher`. Expone `Thresholds`, `Simulator`, `Evaluator`, `Backfiller`, `CatchUp` y `NewScheduler` (catch-up horario, Runner de `NewApplication`); `.SubscribeIndicatorEvaluated()` suscribe a `processing.indicator_evaluated` (traduce contrato → Notice); `.RegisterRoutes(mux)` registra `/api/rule-engine/*` (la simulación usa repos en memoria vía `memorySandbox`). Lee `AGATHA_ENV`, `RULE_ENGINE_BACKFILL_BATCH_SIZE` y `RULE_ENGINE_CATCHUP_MINUTE`. |
| `NewAlertManagementModule(platform)` | Alert Management | `MigrateAlertManagement` (AutoMigrate + `code` GENERATED + índice único parcial + CHECK de gestión + `EXCLUDE` de políticas + trigger) + seed (política global desde 2000-01-01) + repos GORM + `busAlertNotificationPublisher` (port `NotificationPublisher` → bus). Expone `Processor`, `Replay`; `.SubscribeLevelEvaluated()`, `.RegisterRoutes(mux)` (`/api/alerts/*`) y `.Runners()` (catch-up). `NewApplication` lo construye ANTES de Baseline (crea `alerts`, que Baseline lee). Lee `AGATHA_ENV`, `ALERT_MANAGEMENT_BACKFILL_BATCH_SIZE`, `ALERT_MANAGEMENT_CATCHUP_MINUTE`. |
| `NewDashboardModule(platform)` | Dashboard | `.RegisterRoutes(mux)` registra los endpoints `GET /api/dashboard/*`. |
| `NewIngestionApplication()` | Baseline + Ingestion | Composición para `cmd/scheduler`, que arranca el scheduler de Baseline. |

**Entrypoints (`cmd/`)**:

| Binario | Qué ejecuta |
|---|---|
| `cmd/platform` | **Default de la imagen**: monolito con los módulos de `AGATHA_MODULES` (coma-separados: `baseline`, `processing`, `rule_engine`, `alert_management`, `dataos`, `callback`, `dashboard`; vacía = `baseline,callback,dashboard`: `processing`, `rule_engine` y `alert_management` se habilitan a mano). `bootstrap.ParseModules` valida que `processing` requiera `baseline`, que `rule_engine` requiera `processing`, que `alert_management` requiera `rule_engine` y que `callback`/`dataos` sean excluyentes; `bootstrap.NewApplication` cablea solo lo habilitado en orden de arranque (Alert Management migra primero → Baseline → Processing → Rule Engine → Alert Management (suscripción) → DataOS → Callback → Dashboard); las suscripciones quedan registradas antes de arrancar los schedulers. Servidor `:8080` solo si hay módulos HTTP (`callback`, `dashboard`, `rule_engine`, `alert_management`). Apagado ordenado con SIGINT/SIGTERM: `server.Shutdown` → espera a los schedulers → `Platform.Close` (10 s). |
| `cmd/scheduler` | Ingestion programada + scheduler de Baseline. |
| `cmd/baseline` | Recálculo manual de un punto (`-point`, `-asof`, `-dry-run`, `-json`) o carga del histórico retroactivo de versiones (`-backfill`, `-from`); sale al terminar. |
| `cmd/processing` | Evaluación a demanda de un punto (`-point`) o carga del histórico de evaluaciones (`-backfill`, `-from`, `-asof`, `-dry-run`, `-json`); sale al terminar. |
| `cmd/rule_engine` | Histórico de interpretaciones (`-backfill`, `-point`, `-from`, `-to`, `-rebuild`); nunca publica; sale al terminar. |
| `cmd/alert_management` | Histórico de alertas (`-backfill`, `-point`, `-from`, `-to`, `-rebuild`); nunca publica avisos; sale al terminar. |
| `cmd/ingestion` | Ingesta única (rango fijo); sale al terminar. |

**Docker**: `Dockerfile` compila `./cmd/${APP}` (`APP=platform` por defecto) en una imagen `alpine` sin root. `docker-compose.yml` levanta UN servicio `platform` (`AGATHA_MODULES: callback,dashboard,baseline,processing,rule_engine`, `8080:8080`, `env_file: .env`, que debe incluir `AGATHA_CALLBACK_TOKEN` si se habilita `callback`). Cambiar los módulos no requiere rebuild. No se separan módulos en contenedores: el bus en memoria no cruza procesos; separarlos requeriría un bus externo (NATS, Redis Streams, Postgres `LISTEN/NOTIFY`). `.dockerignore` excluye `.env` (los secretos entran en runtime).

---

# Diseños de Base de Datos (Modelos de Telemetría)

### 1. `measuring_points` (Puntos de Medición / Sensores)
- `measuring_point_id` (PK, int): ID del punto de medición (ej. `11556`).
- `name` (string): Nombre del sensor/equipo.
- `active` (boolean): Estado del punto de medición.
- `created_at`, `updated_at` (timestamptz).

### 2. `metrics` (Catálogo de Métricas)
- `id` (PK, int): Identificador único de la métrica (ej. `21` = humedad, `20` = temperatura).
- `key` (string, unique): Clave para código/API (`humedad`, `temperatura`, `pitch`, `rms`).
- `name` (string): Nombre descriptivo (`Humedad (%)`).
- `unit` (string): Unidad de medida (`%`, `°`, `V`, `g/s`).
- `category` (string): Categoría (`energia`, `vibracion`, `orientation`, `espectral`).
- `active` (boolean): Estado de la métrica.
- `created_at`, `updated_at` (timestamptz).

### 2.1 Catalogo de Métricas:

    ID,Key,Nombre,Unidad,Categoría
    1,pitch,Pitch,°,orientation
    2,roll,Roll,°,orientation
    3,voltaje,Voltaje (V),V,energia
    4,corriente,Corriente (mA),mA,energia
    5,bateria,Batería (%),%,energia
    6,rms,RMS (ADC),ADC,vibracion
    7,actividad,Actividad,-,vibracion
    8,mag_promedio,Mag promedio (g),g,acelerometro
    9,act_pico,Act. pico (g/s),g/s,acelerometro
    10,razib_r,Razon R (fuga/viento),-,acustica
    11,b0_viento_30_300hz,B0 viento 30-300Hz,-,espectral
    12,b1_fuga_500_1200hz,B1 fuga 500-1200Hz,-,espectral
    13,b2_fuga_1200_2000hz,B2 fuga 1200-2000Hz,-,espectral
    14,b3_guarda_2_2_4khz,B3 guarda 2-2.4kHz,-,espectral
    15,pitch_base,Pitch base,°,orientation
    16,roll_base,Roll base,°,orientation
    17,rms_acustico,RMS acustico,ADC,acustica
    18,planitud_espectral,Planitud espectral,-,espectral
    19,deriva,Deriva (grados),°,orientation
    20,temperatura,Temperatura (°C),°,temperatura
    21,humedad,Humedad (%),%,humedad

### 3. `raw_measurements` (Serie de Tiempo de Mediciones)
- `id` (PK, `bigint GENERATED ALWAYS AS IDENTITY`): lo genera la BD; el backend nunca lo envía.
- `measuring_point_id` (FK -> `measuring_points.measuring_point_id`): ID del sensor.
- `metric_id` (FK -> `metrics.id`): Métrica asociada.
- `timestamp` (timestamptz): Fecha/hora de la muestra.
- `value` (numeric/float8): Valor numérico registrado (ej. `81.29`).
- `source` (string): Fuente del dato (`dataflow_callback` o `dataos`).
- `ingestion_batch_id` (bigint, NULL, FK -> `ingestion_state_models.id`): lote que trajo la medición (= `batch_seq`; en el callback la une con su `record_id`). NULL en los datos anteriores al 2026-10-29.
- Índice único `ux_raw_measurements_point_metric_ts (measuring_point_id, metric_id, timestamp)`: una medición por punto, métrica e instante; `SaveBatch` usa `ON CONFLICT (...) DO NOTHING`. Es también el índice de las lecturas por punto/métrica/rango (Processing, Baseline, Dashboard).
- Índice `ix_raw_measurements_batch (ingestion_batch_id)`.
- Migración `raw_measurements_pk_unique_batch` (2026-10-29), aplicada a mano en la BD; el modelo GORM replica sus nombres para que `AutoMigrate` no cree nada.

### 4. `ingestion_state_models` (Estado y Control de Ingestión)
> Antes se documentaba como `ingestion_logs`; el nombre real lo genera GORM desde `IngestionStateModel` (sin `TableName()`). El índice conserva el nombre `ux_ingestion_logs_record_id`.
- `id` (PK, int): ID del registro de sincronización.
- `measuring_point_id` (FK -> `measuring_points.measuring_point_id`).
- `source` (string): Origen del lote (ej. `dataos`).
- `last_ingested_at` (timestamptz): Última fecha de datos procesados.
- `received_records` (int): Cantidad de registros ingeridos en el lote.
- `record_id` (varchar(128), NULL): `record_id` del callback de Dataflow Labs; índice único `ux_ingestion_logs_record_id` (NULL en lotes de DataOS). Deduplica reintentos.
- *Append-only*: una fila por lote; el `id` es el `ingestion_batch_id` de cada medición del lote.
- `created_at`, `updated_at` (timestamptz).

### 5. `baseline_versions` (Baseline — versiones de línea base)
- `id` (PK, bigserial), `measuring_point_id` (bigint, **sin FK**), `version` (int). `UNIQUE (measuring_point_id, version)`.
- Vigencia: `valid_from`, `valid_to` (NULL = vigente). Índice único parcial `ux_baseline_versions_current (measuring_point_id) WHERE valid_to IS NULL` ⇒ **una sola vigente por punto**. Índice `ix_baseline_versions_effective (measuring_point_id, valid_from DESC)`.
- Histórico: `history_from`, `history_to`.
- Señal: `median_b2`, `theta_d`, `p90_planitud` (NULL si no hay), `min_elevated_b2` (NULL ⇒ `SIGNAL_DEGENERATE`), `delta_min_b2`, `n_signal_records`, `n_planitud_records`.
- Estimación: `confidence` (`NORMAL`|`LOW`), `exclusion_passes`, `exclusion_converged`, `excluded_ratio`, `excluded_periods` (JSONB `[{from, to}]`, solo fechas), `flags` (TEXT[]), `trigger`, `algorithm_version`, `created_at`.

### 6. `baseline_component_stats` (Baseline — estadísticas por componente)
- PK `(baseline_version_id, window_hours, component)`; FK `baseline_version_id → baseline_versions(id)`.
- `window_hours` (6|24, CHECK), `component` (`D`|`P`|`EX`|`C`, CHECK), `median`, `mad` (crudo, sin 1,4826 ni piso), `n_windows`.

> Ambas tablas las crea `AutoMigrate` en `NewBaselineModule` (los tags del modelo generan índices, FK y CHECKs).

### 7. `feature_evaluations` (Processing — evaluaciones horarias)
- `id` (PK, bigserial), `measuring_point_id` (bigint, **sin FK**), `window_hours` (smallint, `CHECK IN (6, 24)`), `evaluated_at` (hora en punto UTC, fin exclusivo de la ventana), `window_start`.
- `UNIQUE (measuring_point_id, window_hours, evaluated_at)` (`ux_feature_evaluations_point_window_hour`): idempotencia, última hora evaluada y lectura del Rule Engine. ~17 500 filas por dispositivo por año.
- Resultado: `status` (`CALCULATED`|`NO_CALCULABLE`), `no_calc_reason` (`NO_SIGNAL_BASELINE`|`SIGNAL_DEGENERATE`|`LOW_COVERAGE`|`NO_COMPONENT_BASELINE`|`INSUFFICIENT_COHERENCE_DATA`), `n_expected`, `n_received`, `n_valid`, `coverage`.
- Componentes y scores (NULL = no calculado): `d`, `p_hours`, `ex`, `c`, `d_pl`, `c_from_24h`, `z_d`, `z_p`, `z_ex`, `z_c`, `s_d`, `s_p`, `s_ex`, `s_c`, `i`.
- Marcas y trazabilidad: `low_confidence`, `silent`, `flags` (TEXT[]), `baseline_version_id` (sin FK), `baseline_version`, `algorithm_version`, `config_version` (`m=12,13,18`), `trigger` (`RECONCILER`|`MANUAL`|`BACKFILL`), `source_batch_seq` (NULL hoy), `input_max_ts`, `created_at`. Inmutable.

> La crea `AutoMigrate` en `NewProcessingModule`.

### 8. `rule_threshold_versions` (Rule Engine — umbrales versionados, solo se agrega)
- `id` (PK, bigserial), `measuring_point_id` (bigint, **sin FK**), `event_type` (text, default `'FUGA'`), `version` (int). `UNIQUE (measuring_point_id, event_type, version)`.
- Umbrales `amarillo`, `naranja`, `rojo` (numeric(5,4), `CHECK 0 < amarillo < naranja < rojo <= 1`); `min_hours`, `exit_hours` (int ≥ 1); `escalate_amarillo_after_hours`, `escalate_naranja_after_hours` (int NULL = sin escalamiento, si no ≥ 1); `exit_margin` (numeric(5,4), `0 <= exit_margin < amarillo`).
- v2: `confirm_naranja_with_planitud` (bool, default false; `true` exige `d_pl_min`), `d_pl_min` (numeric(5,2) NULL, 0–100), `persistence_mode` (`STRICT`|`WINDOW`, default `STRICT`; `WINDOW` exige `persistence_window_hours` y `persistence_min_hours` ≥ 1 con min ≤ ventana). Los defaults reproducen el comportamiento previo.
- Vigencia `valid_from`, `valid_to` (NULL = vigente; `valid_to > valid_from`). Creación con `SELECT … FOR UPDATE NOWAIT` de la abierta (concurrente ⇒ 409). `EXCLUDE USING gist (measuring_point_id WITH =, event_type WITH =, tstzrange(valid_from, valid_to) WITH &&)` (`btree_gist`). Índice `ix_rule_threshold_versions_effective (measuring_point_id, event_type, valid_from DESC)`.
- Auditoría: `reason`, `created_by`, `created_at`.

### 9. `rule_evaluations` (Rule Engine — interpretación horaria)
- `id` (PK), `measuring_point_id`, `event_type`, `evaluated_at` — `UNIQUE` (`ux_rule_evaluations_point_event_hour`): idempotencia y clave de los eventos. Índice `(measuring_point_id, event_type, evaluated_at DESC)`.
- `indicator_value` (numeric), `raw_level`, `computed_level`, `level` (`NORMAL`|`AMARILLO`|`NARANJA`|`ROJO`), `rule` (`THRESHOLD`|`PERSISTENCE`|`HOLD`|`NONE`), `in_episode`, `hours_above`, `hours_in_level`, `threshold_version_id` (FK → `rule_threshold_versions`), `low_confidence`, `episode_event` (`OPENED`|`CLEARED`|`NONE`), `reasons` (jsonb), `run_amarillo`, `run_naranja`, `below_run` (int, default 0), `d_pl` (double precision NULL), `created_at`. CHECKs en los enums. Una fila por hora **calculada** de 24 h; inmutable (solo `-rebuild` fuera de producción la borra).

> Ambas las crea `persistence.Migrate` en `NewRuleEngineModule` (AutoMigrate + SQL idempotente para `btree_gist` y el `EXCLUDE`).

### 10. Alert Management (`alerts`, `alert_level_segments`, `alert_messages`, `alert_observations`, `alert_processing_state`, `alert_policy_versions`)

- `alerts`: `code` GENERATED (`ALR-` + id con 6 dígitos), `state` `ACTIVA|FINALIZADA` (índice único parcial `ux_alerts_one_active` por punto y evento), `started_at`/`opened_at`/`ended_at`, `current_level`/`max_level`, `opening_indicator_value`/`last_indicator_value`/`max_indicator_value` (el máximo lo citan los mensajes del catálogo v4), contadores (`hours_calculated`, `hours_no_data`, `below_level_hours`), `runtime` (jsonb, memoria de las reglas de mensaje), trazabilidad (`policy_version_id`, `threshold_version_id_at_open`, `opened_by_rule_evaluation_id`), gestión (`management_status`, `classification*`, `label*`, con `*_client_record_id`) con CHECK de coherencia y trigger de inmutabilidad, `source` `LIVE|CATCH_UP|BACKFILL`.
- `alert_level_segments` (colores: `seq`, `opened_by` `OPEN|UP|DOWN|REACTIVATION`, `level`, vigencia), `alert_messages` (solo se agrega; `UNIQUE (alert_id, evaluated_at, code)`; `kind`, texto renderizado, `params`, `catalog_version`; índice `ix_alert_messages_kind`: las `NOTIFICACION` son la fuente de verdad de `alert_management.alert_notification`), `alert_observations` (solo se agrega; `client_record_id` uuid único). FK con `ON DELETE CASCADE` (solo la usa `-rebuild`).
- `alert_processing_state` (PK punto + evento: `last_evaluated_at`, `pending_run_started_at`, `last_alert_id`, `last_status`), `alert_policy_versions` (patrón de `rule_threshold_versions`: global con `measuring_point_id` NULL, `EXCLUDE` por `COALESCE(punto, 0)`). Sin FK a tablas de otros módulos.

