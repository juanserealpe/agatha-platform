# Reglas Fijas y Restricciones (Guardrails)

1. **AISLAMIENTO MODULAR**: Prohibido importar paquetes internos entre módulos (`modules/dashboard` NUNCA importa `modules/ingestion/internal/...`). Comunicación solo vía interfaces públicas/facades en la raíz del módulo o eventos de `shared/eventbus`.
2. **CAPA SHARED OBLIGATORIA**: Prohibido `log.Println`, `json.NewEncoder` directo o `fmt.Errorf` genérico. Es obligatorio usar `shared/logger`, `shared/http` y `shared/errors`.
3. **ACCESO A BD COMPARTIDA**: Una sola BD para el monolito. `ingestion` gestiona la ESCRITURA de telemetría; `dashboard` realiza LECTURA (`SELECT`) de métricas y mediciones para visualización.
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
eventbus.PublishTyped(ctx, bus, contract.TopicMeasurementsIngested, event)
eventbus.SubscribeTyped(bus, topic, handler, log, eventbus.WithName("processing.measurements_ingested"))
```

**Catálogo de eventos**:

| Tópico | Publicador | Consumidores |
|---|---|---|
| `ingestion.measurements_ingested` | Ingestion (post-commit, si se insertó ≥ 1 medición) | Processing (evalúa las horas listas del punto → `feature_evaluations`) |

## Errors (`shared/errors`)

`AppError{Code, Message}` + códigos tipados (`ErrorCode`). Errores de dominio se definen en `domain/errors.go` de cada módulo reutilizando los códigos compartidos.

```go
var ErrCustom = &errors.AppError{Code: errors.CodeNotFound, Message: "recurso no encontrado"}
```

**Códigos disponibles**: `NOT_FOUND`, `INVALID_INPUT`, `FUTURE_DATE`, `ALREADY_INGESTED`, `BEFORE_INITIAL_DATE`, `INVALID_MEASURING_POINT_ID`, `CONSULTING_LAST_STATE`, `GETTING_INGESTED_DATA`, `CONSULTING_DEVICES`, `CONSULTING_MEASUREMENTS`, `CONSULTING_BASELINE`, `PERSISTING_BASELINE`, `CONSULTING_EVALUATIONS`, `PERSISTING_EVALUATIONS`.

## Kernel de cálculo (`shared/features`)

Fórmulas de línea base e índice I compartidas por Baseline y Processing: `Normalize`, `Quantile` (tipo 7), `Median`, `MAD`, `Spearman`, `ComputeComponents`, `MADFloor`, `Score`. Solo stdlib (`depguard` `features-stdlib-only`), sin IO ni estado, versionado con `AlgorithmVersion` (persistido en cada versión de línea base). Detalle: `internal/shared/features/README.md`.

---

# Modulo: Ingestion

**Responsabilidad**: Recibir telemetría y persistirla con control transaccional (Unit of Work). Vía **principal**: callback HTTP de Dataflow Labs (`POST /ingestion/callback`, módulo `callback` de `AGATHA_MODULES`). Vía de **contingencia**: consulta programada a DataOS vía `IngestionScheduler` (módulo `dataos`). No correr ambas a la vez (`raw_measurements` no tiene índice único). Detalle en `modules/ingestion/INGESTION_README.md`.

**Arquitectura**: Hexagonal — `application/ports` define interfaces (`DataProvider`, `IngestionUnitOfWork`, `EventPublisher`), `domain/` contiene entidades y repositorios, `infrastructure/` implementa persistencia, cliente externo DataOS y publicación de eventos (`infrastructure/events`). `contract/` es la superficie pública del módulo (solo stdlib).

## Endpoints / Entry Points

| Trigger | Accion | Servicio |
|---------|--------|----------|
| `POST /ingestion/callback` (Bearer `AGATHA_CALLBACK_TOKEN`) | Valida el registro de Dataflow Labs, mapea sus variables a `metric_id` (`CallbackMapper`; las sin mapeo se avisan con WARN) y persiste en una sola TX el recibo en `ingestion_logs` (idempotente por `record_id`) y las mediciones en `raw_measurements`. Aún no publica `ingestion.measurements_ingested` | `CallbackHandler.ReceiveRecord` → `CallbackService.Receive` |
| `IngestionScheduler.Start()` | Ingesta periódica por intervalo configurable (contingencia) | `IngestService.IngestDataOs(ctx, measuringPointID, start, end)` |

**Callback**: 200 `stored`/`duplicate`; 400 (JSON o campos inválidos), 401, 404 (`mp_id` no registrado), 405, 413 (> 1 MiB), 415, 500 (fallo de BD ⇒ Dataflow Labs reintenta). Timestamps sin zona **o con sufijo `Z`** se interpretan en `America/Bogota` (Dataflow Labs envía la hora local de Colombia marcada como UTC: la misma lectura llega `11:45:41Z` por callback y `16:45:41Z` por DataOS); un offset explícito (`-05:00`) se respeta; `readings` se conserva como literal JSON. Deduplicación: índice único `ux_ingestion_logs_record_id` + `ON CONFLICT (record_id) DO NOTHING` (seguro ante concurrencia).

## DTOs Clave

| DTO | Campos |
|-----|--------|
| `DataRecordCallback` | `event`, `record_id`, `mp_id`, `serial_id`, `timestamp`, `records []{device_id, timestamp, name, readings}` |
| `CallbackResponse` | `status` (`stored`\|`duplicate`), `record_id`, `received_records` |
| `IngestRequest` | `device_id string`, `timestamp time.Time`, `payload map[string]any` |
| `IngestResponse` | `status string`, `message string`, `timestamp time.Time` |

## Entidades de Dominio

- `Device` — `MeasuringPointID`, `Name`, `Active`
- `RawMeasurement` — `Timestamp`, `MeasuringPointID`, `MetricID`, `Value`, `Source`
- `IngestionState` — `MeasuringPointID`, `Source`, `LastIngestedAt`, `ReceivedRecords`, `RecordID` (solo callback)
- `CallbackRecord` / `CallbackReading` — registro del callback validado (timestamps parseados + literales originales)
- `IngestionGap` — `MeasuringPointID`, `StartedAt`, `EndedAt`, `Status` (gap tracking)
- `MetricDefinition` — `Key`, `Name`, `Unit`, `Category`
- `SaveBatchResult` — `Inserted` (filas realmente insertadas por `MeasurementRepository.SaveBatch`)
- `SaveStateResult` — `BatchSeq`, `LastIngestedAt` (devuelto por `IngestionStateRepository.Save`)

## Flujo de Ingesta

`Scheduler` → `IngestService.IngestDataOs` → validar rango → buscar último estado → `DataProvider.GetMeasurements` → `ProcessingService.Process` → `UnitOfWork.Execute` (save batch + save state en TX) → **post-commit**: `EventPublisher.PublishMeasurementsIngested` (solo si `Inserted > 0`; un error se loguea como `Warn` y no falla la ingesta; publisher `nil` = no publica).

> `ProcessingService` es un servicio interno de Ingestion (normalización del lote), distinto del módulo `processing`.

## Eventos Publicados

| Tópico | Evento (`contract/`) | Campos |
|---|---|---|
| `ingestion.measurements_ingested` | `MeasurementsIngested` | `measuring_point_id`, `source`, `from`/`to` (min/max timestamp del lote), `watermark` (`last_ingested_at`), `batch_seq`, `inserted_records`, `occurred_at`, `schema_version` — todos los tiempos en UTC |

`batch_seq` = `id` de la fila insertada en `ingestion_logs` dentro de la misma transacción (la tabla es append-only: una fila por lote), por lo que es monótono y un rollback no lo consume.

`watermark` = `last_ingested_at` = **MAX(timestamp) de las muestras del lote** (`getLatestMeasurementTimestamp` en `IngestService`), NO el fin del rango pedido a DataOS: `watermark ≥ t` garantiza que ya existe un registro con timestamp `≥ t` (supuesto de Processing). Coincide con `to`. Si falla el `Save` del estado, la TX completa se revierte (no quedan mediciones sin su fila de estado) y se devuelve `ErrPersistingIngestionState` (`PERSISTING_INGESTION_STATE`).

---

# Modulo: Dashboard

**Responsabilidad**: Exponer datos de dispositivos y mediciones ya ingeridos para consumo del frontend/dashboard. Solo LECTURA.

**Arquitectura**: Hexagonal — handlers HTTP en `infrastructure/http/`, servicio en `application/services/`, repositorios como interfaces en `domain/repositories/`.

## Endpoints

| Metodo | Handler | Servicio | Descripcion |
|--------|---------|----------|-------------|
| `GET` | `DeviceHandler.GetAllDevices` | `DashboardService.GetAllDevices` | Lista todos los dispositivos con última conexión y batería |
| `GET` | `MeasurementHandler.GetMeasurements` | `DashboardService.GetMeasurements` | Mediciones por `deviceId`, `startDate`, `endDate` (query params RFC3339) |

## DTOs Clave

| DTO | Campos |
|-----|--------|
| `DeviceResponse` | `id int64`, `name string`, `lastConnection *time.Time`, `lastBatteryLevel *float64` |
| `MeasurementsResponse` | `deviceId int64`, `startDate`, `endDate`, `measurements []MetricMeasurementsResponse` |
| `MetricMeasurementsResponse` | `metricId uint64`, `metricName string`, `unit string`, `data []MeasurementPointResponse` |
| `MeasurementPointResponse` | `timestamp time.Time`, `value float64` |

## Validaciones

- `DashboardValidator.ValidateDateRange` — `startDate` debe ser estrictamente anterior a `endDate` (rechaza igualdad).
- `DashboardValidator.ValidateNotFuture(date, err)` — genérica; se aplica a `startDate` y `endDate` por separado, ninguna puede ser futura.
- `deviceId` validado en el handler como entero positivo obligatorio; fechas validadas como RFC3339 obligatorias.
- Existencia del dispositivo verificada antes de consultar mediciones (404 si no existe).
- Rango de consulta `[startDate, endDate)`. Errores de repositorio siempre envueltos en `domain/errors.go` del módulo (`ErrConsultingDevices`, `ErrConsultingMeasurements`) antes de llegar al handler.





# Modulo: Baseline (Device Context & Baseline)

**Responsabilidad**: mantener por dispositivo, versionada en el tiempo, la línea base estadística (señal: `mediana_B2`, `θ_D = P90_B2`, `P90_Planitud`; componentes: mediana y MAD de D, P, Ex en 6 h/24 h y C en 24 h) con estimación iterativa que excluye periodos anómalos (`I(24 h) ≥ 0,18`, tope 40 %, máx. 5 pasadas). Detalle en `modules/baseline/BASELINE_README.md`.

**Eventos conocidos**: los intervalos de fugas/intervenciones conocidas se excluyen siempre del cálculo (además de lo que marca el barrido) y la versión lleva el flag `KNOWN_EVENTS_EXCLUDED`. Los entrega el port `KnownEventsProvider`; hoy es una lista **estática en código** (`infrastructure/knownevents`, solo punto 11556), a reemplazar por un adapter del módulo de alertas en `internal/bootstrap`.

**Contrato público (congelado para Processing)**: `baseline.Facade.GetEffectiveBaseline(ctx, pointID, at)` → `EffectiveBaseline` (`types.go`, solo stdlib). Vigencia `[valid_from, valid_to)` no retroactiva; sin versión ⇒ `Status = NONE` (no es error).

| Trigger | Accion | Componente |
|---|---|---|
| `RecalcScheduler` (al arrancar + cada hora en `BASELINE_TICK_MINUTE`) | Recalcula los puntos activos a los que les toca (sin versión ⇒ `BOOTSTRAP`; `LOW`/`NO_EXCLUSION_HISTORY` cada 24 h; resto cada 168 h) | `RecalculationService.RecalculateDue` → `Recalculate` |
| `cmd/baseline -point -asof [-dry-run] [-json]` | Recálculo manual / validación con el analista | `RecalculationService.Recalculate` |
| `cmd/baseline -point -backfill [-from] [-asof] [-dry-run] [-json]` | Histórico retroactivo: persiste, en una TX, las versiones que habrían existido desde el primer registro (política real, `created_at = valid_from`). Solo en un punto sin versiones | `BackfillService.Backfill` → `ReplayService.SimulateVersions` + `SaveHistory` |
| `Facade.GetEffectiveBaseline` | Versión vigente en `at` + `first_seen_at` | `QueryService.GetEffective` |
| `Replayer.SimulateVersions` / `ComputeAt` (solo `cmd/replay`) | Versiones que habrían existido en `[from, to)` (reloj virtual horario, política real, solo datos anteriores a cada recálculo) o una versión en `asOf`; nunca escribe | `ReplayService` |

**Aislamiento**: solo LEE `raw_measurements` y `devices` (modelo de lectura propio, sin FK a tablas de Ingestion) y ESCRIBE `baseline_versions` / `baseline_component_stats`. No importa `ingestion`, `processing` ni `shared/eventbus` (`depguard` `baseline-isolation`). No publica ni consume eventos.

**Errores**: `ErrInvalidMeasuringPointID`, `ErrConsultingHistory` (`CONSULTING_MEASUREMENTS`), `ErrConsultingBaseline`, `ErrPersistingBaseline`, `ErrBaselineConcurrentUpdate` (no-op), `ErrConsultingKnownEvents` (`CONSULTING_BASELINE`: sin la lista de eventos no se calcula), `ErrBaselineHistoryExists` (`INVALID_INPUT`: backfill sobre un punto con versiones). `INSUFFICIENT_HISTORY` y los flags no son errores.

---

# Modulo: Processing

**Responsabilidad**: cada hora y para cada punto activo, evaluar las ventanas de 24 h y 6 h que terminan en esa hora contra la línea base vigente y persistir **una fila por (punto, ventana, hora)** en `feature_evaluations` (Cobertura, D, P, Ex, C, D_Pl, z, s(z), I) o el motivo `NO_CALCULABLE`. **Mide, no interpreta**: rachas, cortes, categorías y alertas son del Rule Engine. Todo cálculo pasa por `shared/features`. Detalle en `modules/processing/PROCESSING_README.MD`.

| Trigger | Accion | Componente |
|---|---|---|
| Evento `ingestion.measurements_ingested` | Evalúa las horas listas del punto (máx. 24) | `MeasurementsIngestedSubscriber.Handle` → `EvaluationService.EvaluatePending` |
| `HourlyScheduler` (al arrancar + minuto 35) | Reconciliador: completa las horas sin fila de cada punto activo (máx. 72 por tick; grace 30 min) | `EvaluationService.Reconcile` |
| `cmd/replay -point -from -to -mode asof\|fixed` | Reproducción histórica a CSV, sin escribir en la BD | `bootstrap.RunReplay` |

**Garantías**: idempotente (índice único + `ON CONFLICT DO NOTHING`, lock en proceso por punto); una TX por lote (todas las horas o ninguna); un fallo de lectura o de Baseline **no** persiste filas `NO_CALCULABLE` falsas (aborta y el reconciliador reintenta); toda hora `≤ floorHour(now − 30 min)` tiene fila tras el tick del reconciliador.

**Errores**: `ErrInvalidMeasuringPointID`, `ErrConsultingMeasurements`, `ErrConsultingBaseline`, `ErrConsultingEvaluations` (`CONSULTING_EVALUATIONS`), `ErrPersistingEvaluations` (`PERSISTING_EVALUATIONS`). `NO_CALCULABLE`, ausencia de línea base y conflicto de unicidad no son errores. Subscriber y scheduler registran `code` + mensaje y no reintentan.

**Aislamiento**: solo importa `shared/{logger, errors, features}`; no importa `ingestion`, `baseline` ni `shared/eventbus` (`depguard` `processing-isolation`). Lee `raw_measurements` y `devices` con su propio modelo y consume Baseline vía su port `BaselineProvider`; el adapter `baselineProviderAdapter`, la traducción del evento y la suscripción viven en `internal/bootstrap/processing.go`.

**Inicialización**: `bootstrap.NewProcessingModule(platform, baselineModule)`. Debe correr en el MISMO proceso que Ingestion (bus en memoria) y antes de que se publique. `toMeasurementsIngestedNotice` mapea `watermark` → `Watermark` (hora lista = `floorHour(watermark)`), `batch_seq` → `BatchSeq` y `from` → `From`; si el evento no trae `watermark` (cero), la hora lista se deriva de `MAX(timestamp)` en la BD.

---

# Composition Root y Despliegue

`internal/bootstrap` arma cada módulo sobre una `Platform` compartida (`NewPlatform`: `.env` + `DATABASE_URL` + bus en memoria). Cada módulo tiene su constructor y ninguno conoce a los demás:

| Constructor | Módulo | Notas |
|---|---|---|
| `NewPlatform()` | Shared | BD única + `InMemoryBus`. `Platform.Close(ctx)` drena el bus. |
| `NewBaselineModule(platform)` | Baseline | AutoMigrate (tablas + índice parcial vía tags) + config `BASELINE_*`. Expone `Facade`, `Recalculator`, `Backfiller` y `NewScheduler()`. |
| `NewIngestionModule(platform)` | Ingestion (DataOS) | AutoMigrate + DataOS + publisher. `.NewScheduler()` crea el scheduler (punto 11556, cada 15 min, +50 s). |
| `NewCallbackModule(platform)` | Ingestion (callback) | AutoMigrate (helper compartido `migrateIngestionSchema`) + `CallbackService` + handler. Lee `AGATHA_CALLBACK_TOKEN` (vacía ⇒ arranca y responde 401 a todo). No requiere credenciales de DataOS. `.RegisterRoutes(mux)` registra `/ingestion/callback`. |
| `NewProcessingModule(platform, baseline)` | Processing | AutoMigrate (`feature_evaluations` + índice único vía tags) + config `PROCESSING_*` + adapter sobre `baseline.Facade` + subscriber. Expone `NewScheduler()` (reconciliador). Llamar antes de que Ingestion publique. |
| `NewDashboardModule(platform)` | Dashboard | `.RegisterRoutes(mux)` registra los endpoints `GET /api/dashboard/*`. |
| `NewIngestionApplication()` | Baseline + Processing + Ingestion | Composición para `cmd/scheduler` y `cmd/ingestion` (solo `cmd/scheduler` arranca los schedulers de Baseline y Processing). |
| `NewReadOnlyPlatform()` / `RunReplay(opts)` | Processing + Baseline (`Replayer`) | Composición de solo lectura para `cmd/replay`: sin AutoMigrate, sin suscripciones ni schedulers, sesión `default_transaction_read_only=on`. |

**Entrypoints (`cmd/`)**:

| Binario | Qué ejecuta |
|---|---|
| `cmd/platform` | **Default de la imagen**: monolito con los módulos de `AGATHA_MODULES` (coma-separados: `baseline`, `processing`, `dataos`, `callback`, `dashboard`; vacía = `baseline,processing,callback,dashboard`). `bootstrap.ParseModules` valida que `processing` requiera `baseline` y que `callback`/`dataos` sean excluyentes; `bootstrap.NewApplication` cablea solo lo habilitado en orden de arranque (Baseline → Processing → DataOS → Callback → Dashboard). Servidor `:8080` solo si hay módulos HTTP (`callback`/`dashboard`). Apagado ordenado con SIGINT/SIGTERM: `server.Shutdown` → espera a los schedulers → `Platform.Close` (10 s). |
| `cmd/api` | Solo Dashboard (`:8080`). |
| `cmd/scheduler` | Ingestion programada + Processing (subscriber + reconciliador) + scheduler de Baseline. |
| `cmd/baseline` | Recálculo manual de un punto (`-point`, `-asof`, `-dry-run`, `-json`) o carga del histórico retroactivo de versiones (`-backfill`, `-from`); sale al terminar. |
| `cmd/ingestion` | Ingesta única (rango fijo) + Processing sin reconciliador (el evento evalúa las horas listas); sale al terminar. |
| `cmd/replay` | Reproducción histórica de Processing a CSV (`-point`, `-from`, `-to`, `-mode asof\|fixed`, `-fixed-asof`, `-out`); solo lectura; sale al terminar. |

**Docker**: `Dockerfile` compila `./cmd/${APP}` (`APP=platform` por defecto) en una imagen `alpine` sin root. `docker-compose.yml` levanta UN servicio `platform` (`AGATHA_MODULES: callback,dashboard`, `8080:8080`, `env_file: .env`, que debe incluir `AGATHA_CALLBACK_TOKEN` si se habilita `callback`). Cambiar los módulos no requiere rebuild. No se separan módulos en contenedores: el bus en memoria no cruza procesos; separarlos requeriría un bus externo (NATS, Redis Streams, Postgres `LISTEN/NOTIFY`). `.dockerignore` excluye `.env` (los secretos entran en runtime).

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
- `unit` (string): Unidad de medida (`%`, `°`, `mV`, `g/s`).
- `category` (string): Categoría (`energia`, `vibracion`, `orientation`, `espectral`).
- `active` (boolean): Estado de la métrica.
- `created_at`, `updated_at` (timestamptz).

### 2.1 Catalogo de Métricas:

    ID,Key,Nombre,Unidad,Categoría
    1,pitch,Pitch,°,orientation
    2,roll,Roll,°,orientation
    3,voltaje,Voltaje (mV),mV,energia
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

### 3. `measurements` (Serie de Tiempo de Mediciones)
- `measuring_point_id` (FK -> `measuring_points.measuring_point_id`): ID del sensor.
- `metric_id` (FK -> `metrics.id`): Métrica asociada.
- `timestamp` (timestamptz): Fecha/hora de la muestra.
- `value` (numeric/float8): Valor numérico registrado (ej. `81.29`).
- `source` (string): Fuente del dato (ej. `dataos`).
*Índice Sugerido:* Composite PK (`measuring_point_id`, `metric_id`, `timestamp`).

### 4. `ingestion_logs` (Estado y Control de Ingestión)
- `id` (PK, int): ID del registro de sincronización.
- `measuring_point_id` (FK -> `measuring_points.measuring_point_id`).
- `source` (string): Origen del lote (ej. `dataos`).
- `last_ingested_at` (timestamptz): Última fecha de datos procesados.
- `received_records` (int): Cantidad de registros ingeridos en el lote.
- `record_id` (varchar(128), NULL): `record_id` del callback de Dataflow Labs; índice único `ux_ingestion_logs_record_id` (NULL en lotes de DataOS). Deduplica reintentos.
- *Append-only*: una fila por lote; el `id` se usa como `batch_seq` (cursor monótono) en el evento `MeasurementsIngested`.
- `created_at`, `updated_at` (timestamptz).

### 5. `baseline_versions` (Baseline — versiones de línea base)
- `id` (PK, bigserial), `measuring_point_id` (bigint, **sin FK**), `version` (int). `UNIQUE (measuring_point_id, version)`.
- Vigencia: `valid_from`, `valid_to` (NULL = vigente). Índice único parcial `ux_baseline_versions_current (measuring_point_id) WHERE valid_to IS NULL` ⇒ **una sola vigente por punto**. Índice `ix_baseline_versions_effective (measuring_point_id, valid_from DESC)`.
- Histórico: `history_from`, `history_to`.
- Señal: `median_b2`, `theta_d`, `p90_planitud` (NULL si no hay), `min_elevated_b2` (NULL ⇒ `SIGNAL_DEGENERATE`), `delta_min_b2`, `n_signal_records`, `n_planitud_records`.
- Estimación: `confidence` (`NORMAL`|`LOW`), `exclusion_passes`, `exclusion_converged`, `excluded_ratio`, `excluded_periods` (JSONB `[{from, to, peak_i}]`), `flags` (TEXT[]), `trigger`, `algorithm_version`, `created_at`.

### 6. `baseline_component_stats` (Baseline — estadísticas por componente)
- PK `(baseline_version_id, window_hours, component)`; FK `baseline_version_id → baseline_versions(id)`.
- `window_hours` (6|24, CHECK), `component` (`D`|`P`|`EX`|`C`, CHECK), `median`, `mad` (crudo, sin 1,4826 ni piso), `n_windows`.

> Ambas tablas las crea `AutoMigrate` en `NewBaselineModule` (los tags del modelo generan índices, FK y CHECKs).

### 7. `feature_evaluations` (Processing — evaluaciones horarias)
- `id` (PK, bigserial), `measuring_point_id` (bigint, **sin FK**), `window_hours` (smallint, `CHECK IN (6, 24)`), `evaluated_at` (hora en punto UTC, fin exclusivo de la ventana), `window_start`.
- `UNIQUE (measuring_point_id, window_hours, evaluated_at)` (`ux_feature_evaluations_point_window_hour`): idempotencia, última hora evaluada y lectura del Rule Engine. ~17 500 filas por dispositivo por año.
- Resultado: `status` (`CALCULATED`|`NO_CALCULABLE`), `no_calc_reason` (`NO_SIGNAL_BASELINE`|`SIGNAL_DEGENERATE`|`LOW_COVERAGE`|`NO_COMPONENT_BASELINE`|`INSUFFICIENT_COHERENCE_DATA`), `n_expected`, `n_received`, `n_valid`, `coverage`.
- Componentes y scores (NULL = no calculado): `d`, `p_hours`, `ex`, `c`, `d_pl`, `c_from_24h`, `z_d`, `z_p`, `z_ex`, `z_c`, `s_d`, `s_p`, `s_ex`, `s_c`, `i`.
- Marcas y trazabilidad: `low_confidence`, `silent`, `flags` (TEXT[]), `baseline_version_id` (sin FK), `baseline_version`, `algorithm_version`, `config_version` (`m=12,13,18`), `trigger` (`EVENT`|`RECONCILER`), `source_batch_seq`, `input_max_ts`, `created_at`. Inmutable.

> La crea `AutoMigrate` en `NewProcessingModule`.
