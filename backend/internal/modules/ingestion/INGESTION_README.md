# Módulo Ingestion

**Responsabilidad**: recibir la telemetría de los puntos de medición y persistirla con control transaccional (Unit of Work). Es el único módulo que ESCRIBE telemetría (`raw_measurements`, `ingestion_logs`).

Hay dos vías de entrada:

| Vía | Rol | Entrypoint |
|---|---|---|
| Callback HTTP de Dataflow Labs (`POST /ingestion/callback`) | **Principal** (default de Docker) | `cmd/platform` con `AGATHA_MODULES` incluyendo `callback` |
| Consulta programada a DataOS (`IngestionScheduler`, cada 15 min) | **Contingencia** | `cmd/platform` con `AGATHA_MODULES` incluyendo `dataos`, `cmd/scheduler`, `cmd/ingestion` |

No corras ambas a la vez: `raw_measurements` no tiene índice único y la consulta a DataOS duplicaría filas.

**Arquitectura**: hexagonal. `application/ports` define `DataProvider`, `IngestionUnitOfWork` y `EventPublisher`; `domain/` tiene entidades, repositorios y errores; `infrastructure/` implementa la persistencia (GORM), el cliente DataOS, el handler HTTP del callback (`infrastructure/http`) y la publicación de eventos. `contract/` es la superficie pública (solo stdlib).

## Endpoints / Entry Points

| Trigger | Acción | Servicio |
|---|---|---|
| `POST /ingestion/callback` | Recibe un registro de Dataflow Labs, lo registra en el log y persiste su recibo (idempotente por `record_id`) | `CallbackHandler.ReceiveRecord` → `CallbackService.Receive` |
| `IngestionScheduler.Start()` | Ingesta periódica desde DataOS | `IngestService.IngestDataOs(ctx, measuringPointID, start, end)` |

## Callback de Dataflow Labs

### Contrato

- `POST /ingestion/callback` en `:8080`, `Content-Type: application/json` (se admite `; charset=...`).
- `Authorization: Bearer <token>`, comparado en tiempo constante con `AGATHA_CALLBACK_TOKEN`. El token nunca se registra ni se devuelve.
- Cuerpo máximo: 1 MiB (`MaxCallbackBodyBytes`). Un único objeto JSON; los campos desconocidos se ignoran.
- Tiempo máximo de procesamiento: 8 s (Dataflow Labs espera 2xx en < 10 s).

```json
{
  "event": "data_record",
  "record_id": "5d0c1f0e8a7b4c3d9e2f1a0b6c7d8e9f",
  "mp_id": 11556,
  "serial_id": "AGATHA02",
  "timestamp": "2026-10-08T16:15:41",
  "records": [
    { "device_id": 11556, "timestamp": "2026-10-08T16:15:41", "name": "env_temperature_c", "readings": 22.4 }
  ]
}
```

### Validación (`CallbackValidator`)

- `event` debe ser exactamente `data_record`.
- `record_id` es obligatorio y tiene como máximo 128 caracteres. Es estable entre reintentos.
- `mp_id` es un entero positivo. `serial_id` es obligatorio.
- `records` debe tener al menos una variable. Ninguna variable es obligatoria (por ejemplo, `accel_drift_deg_cloud` puede faltar).
- En cada variable: `device_id == mp_id`, `name` obligatorio, `readings` obligatorio y numérico (rechaza texto, `null` y booleanos).
- Timestamps: RFC 3339 con zona (`...Z`, `...-05:00`) o sin zona (`2006-01-02T15:04:05[.fff]`), que se interpreta en **America/Bogota**, la misma convención que se usa al consultar DataOS.
- `readings` se conserva como el **literal JSON original** (`dtos.Reading`, por ejemplo `"0.0"` o `"77.01"`), sin pasar por `float64`.

### Respuestas

| Status | Cuándo |
|---|---|
| `200` `{"status":"stored"}` | Registro nuevo persistido |
| `200` `{"status":"duplicate"}` | `record_id` ya procesado (reintento) |
| `400` `INVALID_INPUT` | JSON mal formado, tipos inválidos, contenido extra o campos inválidos |
| `401` `UNAUTHORIZED` | Falta la cabecera, el esquema no es Bearer, el token es incorrecto o `AGATHA_CALLBACK_TOKEN` no está configurada (incluye `WWW-Authenticate`) |
| `404` `NOT_FOUND` | `mp_id` no existe en `devices` |
| `405` `METHOD_NOT_ALLOWED` | Método distinto de POST (incluye `Allow: POST`) |
| `413` `PAYLOAD_TOO_LARGE` | Cuerpo de más de 1 MiB |
| `415` `UNSUPPORTED_MEDIA_TYPE` | `Content-Type` distinto de `application/json` |
| `500` | Fallo al consultar `devices` o al persistir: Dataflow Labs debe reintentar |

### Persistencia e idempotencia

- Cada registro nuevo es **una fila en `ingestion_logs`** (`IngestionStateModel`) con `source = dataflow_callback`, `record_id`, `received_records = len(records)` y `last_ingested_at` = MAX(timestamp) de las variables.
- `record_id` tiene un índice único (`ux_ingestion_logs_record_id`; los lotes de DataOS lo dejan en NULL). `IngestionStateRepository.SaveIfAbsent` inserta con `ON CONFLICT (record_id) DO NOTHING`: si llegan dos peticiones concurrentes con el mismo `record_id`, la segunda espera el commit de la primera y responde `duplicate`. Si la primera se revierte, la segunda inserta.
- **Etapa actual (observación)**: el callback **todavía no mapea** las variables (`env_temperature_c`, `accel_drift_deg_cloud`, ...) a `metric_id` ni escribe `raw_measurements`, así que no publica `ingestion.measurements_ingested`. Cada variable se registra en el log (`[Callback] variable recibida`, con `name`, `readings`, `device_id` y `timestamp`) para confirmar qué llega. Cuando exista el mapeo, las mediciones se guardarán dentro de la misma TX, después de reclamar el `record_id`.

## Flujo de ingesta DataOS (contingencia)

`Scheduler` → `IngestService.IngestDataOs` → validar rango → buscar el último estado → `DataProvider.GetMeasurements` → `ProcessingService.Process` → `UnitOfWork.Execute` (guardar el lote y el estado en una TX) → **post-commit**: `EventPublisher.PublishMeasurementsIngested` (solo si `Inserted > 0`; un error se registra como `Warn` y no hace fallar la ingesta).

## DTOs clave

| DTO | Campos |
|---|---|
| `DataRecordCallback` | `event`, `record_id`, `mp_id int64`, `serial_id`, `timestamp string`, `records []CallbackReading` |
| `CallbackReading` | `device_id int64`, `timestamp string`, `name`, `readings Reading` (literal numérico) |
| `CallbackResponse` | `status` (`stored`/`duplicate`), `record_id`, `received_records` |
| `IngestRequest` | `device_id string`, `timestamp time.Time`, `payload map[string]any` |
| `IngestResponse` | `status string`, `message string`, `timestamp time.Time` |

## Entidades de dominio

- `Device`: `MeasuringPointID`, `Name`, `Active`
- `RawMeasurement`: `Timestamp`, `MeasuringPointID`, `MetricID`, `Value`, `Source`
- `IngestionState`: `MeasuringPointID`, `Source`, `LastIngestedAt`, `ReceivedRecords`, `RecordID *string` (solo para el callback)
- `CallbackRecord` / `CallbackReading`: registro del callback ya validado (timestamps parseados, `RawTimestamp` y `Value` literales)
- `IngestionGap`, `MetricDefinition`, `SaveBatchResult`, `SaveStateResult`

## Errores (`domain/errors.go`)

`ErrPersistingIngestionState` (`PERSISTING_INGESTION_STATE` → 500), `ErrConsultingDevice` (`CONSULTING_DEVICES` → 500), `NewInvalidCallbackPayloadError(detail)` (`INVALID_INPUT` → 400) y los códigos locales del callback: `UNAUTHORIZED`, `METHOD_NOT_ALLOWED`, `PAYLOAD_TOO_LARGE`, `UNSUPPORTED_MEDIA_TYPE`. El handler responde estos últimos directamente porque `shared/http` no los mapea.

## Eventos publicados

| Tópico | Evento (`contract/`) | Publicado por |
|---|---|---|
| `ingestion.measurements_ingested` | `MeasurementsIngested` | Solo la ingesta DataOS (post-commit, si se insertó al menos 1 medición). El callback no lo publica todavía porque no escribe mediciones. |
