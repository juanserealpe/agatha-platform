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
| `ingestion.measurements_ingested` | Ingestion (post-commit, si se insertó ≥ 1 medición) | Processing (lee la última ingesta del punto y la registra en log) |

## Errors (`shared/errors`)

`AppError{Code, Message}` + códigos tipados (`ErrorCode`). Errores de dominio se definen en `domain/errors.go` de cada módulo reutilizando los códigos compartidos.

```go
var ErrCustom = &errors.AppError{Code: errors.CodeNotFound, Message: "recurso no encontrado"}
```

**Códigos disponibles**: `NOT_FOUND`, `INVALID_INPUT`, `FUTURE_DATE`, `ALREADY_INGESTED`, `BEFORE_INITIAL_DATE`, `INVALID_MEASURING_POINT_ID`, `CONSULTING_LAST_STATE`, `GETTING_INGESTED_DATA`, `CONSULTING_DEVICES`, `CONSULTING_MEASUREMENTS`.

---

# Modulo: Ingestion

**Responsabilidad**: Ingestar mediciones desde DataOS, procesarlas y persistirlas con control transaccional (Unit of Work). Ejecuta ingestas programadas vía `IngestionScheduler`.

**Arquitectura**: Hexagonal — `application/ports` define interfaces (`DataProvider`, `IngestionUnitOfWork`, `EventPublisher`), `domain/` contiene entidades y repositorios, `infrastructure/` implementa persistencia, cliente externo DataOS y publicación de eventos (`infrastructure/events`). `contract/` es la superficie pública del módulo (solo stdlib).

## Endpoints / Entry Points

| Trigger | Accion | Servicio |
|---------|--------|----------|
| `IngestionScheduler.Start()` | Ingesta periódica por intervalo configurable | `IngestService.IngestDataOs(ctx, measuringPointID, start, end)` |

## DTOs Clave

| DTO | Campos |
|-----|--------|
| `IngestRequest` | `device_id string`, `timestamp time.Time`, `payload map[string]any` |
| `IngestResponse` | `status string`, `message string`, `timestamp time.Time` |

## Entidades de Dominio

- `Device` — `MeasuringPointID`, `Name`, `Active`
- `RawMeasurement` — `Timestamp`, `MeasuringPointID`, `MetricID`, `Value`, `Source`
- `IngestionState` — `MeasuringPointID`, `Source`, `LastIngestedAt`, `ReceivedRecords`
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





# Modulo: Processing

**Responsabilidad**: Reaccionar a los lotes de mediciones ingeridos. Detalle en `modules/processing/README.MD`.

**Estado actual**: consume `ingestion.measurements_ingested` y lee de la BD (solo lectura) la última fecha ingerida del punto de medición con todas sus variables; el resultado solo se registra en log. No escribe BD.

| Trigger | Accion | Componente |
|---|---|---|
| Evento `ingestion.measurements_ingested` | Lee `MAX(timestamp)` de `raw_measurements` del punto + variables de esa fecha (JOIN `metric_definitions`) → log | `MeasurementsIngestedSubscriber.Handle` → `LatestIngestionService.GetLatestIngestion` → `MeasurementRepository.GetLatestMeasurements` |

**Errores**: `processing/domain/errors.go` (`ErrInvalidMeasuringPointID`, `ErrConsultingLatestMeasurements`, `ErrLatestMeasurementsNotFound`) reutilizando códigos de `shared/errors`. Al no haber HTTP, el subscriber registra el `AppError` (`code` + mensaje) en log y no reintenta.

**Aislamiento**: Processing NO importa `modules/ingestion/...` (tampoco sus entidades: tiene su propia vista `RawMeasurement`/`MetricDefinition` de las mismas tablas). `internal/bootstrap/processing.go` traduce `contract.MeasurementsIngested` → `MeasurementsIngestedNotice` y registra la suscripción `processing.measurements_ingested`.

**Inicialización**: `bootstrap.NewProcessingModule(platform)`. Debe correr en el MISMO proceso que Ingestion (bus en memoria) y antes de que se publique.

---

# Composition Root y Despliegue

`internal/bootstrap` arma cada módulo sobre una `Platform` compartida (`NewPlatform`: `.env` + `DATABASE_URL` + bus en memoria). Cada módulo tiene su constructor y ninguno conoce a los demás:

| Constructor | Módulo | Notas |
|---|---|---|
| `NewPlatform()` | Shared | BD única + `InMemoryBus`. `Platform.Close(ctx)` drena el bus. |
| `NewIngestionModule(platform)` | Ingestion | AutoMigrate + DataOS + publisher. `.NewScheduler()` crea el scheduler (punto 11556, cada 15 min, +50 s). |
| `NewProcessingModule(platform)` | Processing | Registra subscribers. Llamar antes de que Ingestion publique. |
| `NewDashboardModule(platform)` | Dashboard | `.RegisterRoutes(mux)` registra los endpoints `GET /api/dashboard/*`. |
| `NewIngestionApplication()` | Ingestion + Processing | Composición para `cmd/scheduler` y `cmd/ingestion`. |

**Entrypoints (`cmd/`)**:

| Binario | Qué ejecuta |
|---|---|
| `cmd/platform` | **Monolito completo** (default de la imagen): scheduler de Ingestion + Processing + API de Dashboard en `:8080`. Apagado ordenado con SIGINT/SIGTERM: `server.Shutdown` → espera al scheduler → `Platform.Close` (10 s). |
| `cmd/api` | Solo Dashboard (`:8080`). |
| `cmd/scheduler` | Solo Ingestion programada + Processing. |
| `cmd/ingestion` | Ingesta única (rango fijo) + Processing; sale al terminar. |

**Docker**: `Dockerfile` compila `./cmd/${APP}` (`APP=platform` por defecto) en una imagen `alpine` sin root. `docker-compose.yml` levanta UN servicio `platform` (`8080:8080`, `env_file: .env`). No se separan módulos en contenedores: el bus en memoria no cruza procesos; separarlos requeriría un bus externo (NATS, Redis Streams, Postgres `LISTEN/NOTIFY`). `.dockerignore` excluye `.env` (los secretos entran en runtime).

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
- *Append-only*: una fila por lote; el `id` se usa como `batch_seq` (cursor monótono) en el evento `MeasurementsIngested`.
- `created_at`, `updated_at` (timestamptz).