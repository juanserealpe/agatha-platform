# Módulo Ingestion

**Responsabilidad**: recibir la telemetría de los puntos de medición y persistirla con control transaccional (Unit of Work). Es el único módulo que ESCRIBE telemetría (`raw_measurements`, `ingestion_state_models`).

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
| `POST /ingestion/callback` | Recibe un registro de Dataflow Labs, mapea sus variables a métricas y persiste en una TX el recibo (idempotente por `record_id`) y las mediciones en `raw_measurements` | `CallbackHandler.ReceiveRecord` → `CallbackService.Receive` |
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

- Cada registro nuevo es **una fila en `ingestion_state_models`** (`IngestionStateModel`; la documentación vieja la llamaba `ingestion_logs`, pero `IngestionStateModel` no define `TableName()` y GORM usa ese nombre) con `source = dataflow_callback`, `record_id`, `received_records = len(records)` y `last_ingested_at` = MAX(timestamp) de las variables.
- `record_id` tiene un índice único (`ux_ingestion_logs_record_id`; los lotes de DataOS lo dejan en NULL). `IngestionStateRepository.SaveIfAbsent` inserta con `ON CONFLICT (record_id) DO NOTHING`: si llegan dos peticiones concurrentes con el mismo `record_id`, la segunda espera el commit de la primera y responde `duplicate`. Si la primera se revierte, la segunda inserta.
- **Mediciones en la misma TX**: dentro del `IngestionUnitOfWork` primero se reclama el `record_id` (`SaveIfAbsent`) y, solo si es nuevo, se escriben las mediciones con `MeasurementRepository.SaveBatch` (`source = dataflow_callback`, `timestamp` = el de cada variable). Un reintento (`duplicate`) no reescribe mediciones. Si `SaveBatch` falla, se revierte también el recibo y se responde 500 para que Dataflow Labs reintente.
- **Todavía no publica** `ingestion.measurements_ingested` (el módulo `callback` no cablea `EventPublisher`).

### Mapeo de variables (`CallbackMapper`)

`callbackMetricIDMap` traduce el `name` de cada variable al `metric_id` del catálogo (`ARQUITECTURA_LITE.md` §2.1), equivalente al mapper de DataOS (`external/dataos/mapping/mapper.go`). `readings` se convierte a `float64` (si no cabe ⇒ 400).

| Variable | metric_id | Métrica |
|----------|-----------|---------|
| `accel_pitch` | 1 | Pitch |
| `accel_roll` | 2 | Roll |
| `power_vbat_mv` | 3 | Voltaje (V) |
| `power_ibat_ma` | 4 | Corriente (mA) |
| `power_battery_pct` | 5 | Batería (%) |
| `piezo_rms` | 6 | RMS (ADC) |
| `piezo_activity` | 7 | Actividad (piezoeléctrica, %) |
| `accel_mag_avg` | 8 | Mag promedio (g) |
| `accel_activity_peak` | 9 | Act. pico (g/s) |
| `pspec_ratio` | 10 | Razon R (fuga/viento) |
| `pspec_b0_wind` | 11 | B0 viento 30-300Hz |
| `pspec_b1_leak` | 12 | B1 fuga 500-1200Hz |
| `pspec_b2_leak` | 13 | B2 fuga 1200-2000Hz |
| `pspec_b3_guard` | 14 | B3 guarda 2-2.4kHz |
| `accel_base_pitch` | 15 | Pitch base |
| `accel_base_roll` | 16 | Roll base |
| `pspec_rms` | 17 | RMS acustico |
| `pspec_flatness` | 18 | Planitud espectral |
| `accel_drift_deg_cloud` | 19 | Deriva (grados) — NO `accel_drift_deg` |
| `env_temperature_c` | 20 | Temperatura (°C) |
| `env_humidity_pct` | 21 | Humedad (%) |

Criterio RMS/Actividad: en el análisis acústico del analista "RMS acústico" ronda 5 (= `pspec_rms`; `piezo_rms` ronda 56) y "Actividad" es la actividad piezoeléctrica en %.

Todo lo que está fuera de las 21 métricas se **omite** (no se guarda): `accel_activity_avg`, `accel_movement`, `accel_shock`, `accel_{x,y,z}_{avg,min,max,std}`, `accel_vibration`, `accel_event`, `accel_drift_deg`, `accel_drift_alarm[_cloud]`, `accel_drift_run[_cloud]`, `accel_settle[_cloud]`, `accel_baseline_valid[_cloud]`, `piezo_peak`, `piezo_leak_{suspected,confirmed}`, `power_energy_mwh`, `power_modem_{ma_avg,duty_pct}`, `power_p_{in,out}`, `samples`, `pspec_centroid`, `pspec_leak_suspect`. Se informan en un único `WARN` por registro: `variables sin mapeo; no se guardan` (`record_id`, `variables`); el registro igual se acepta.

### Normalización de valores (callback y DataOS)

Antes de persistir, ambos mappers pasan el valor por `entities.NormalizeMeasurementValue(metricID, value)` (`domain/entities/raw_measurement.go`). Regla vigente: **Voltaje (`metric_id = 3`) se divide entre 2** (la fuente lo entrega al doble). El resto de métricas se guarda tal cual. El payload crudo del callback no se toca; solo `raw_measurements.value`. Datos anteriores al cambio (2026-10-22) se corrigieron con un `UPDATE` manual sobre `raw_measurements`.

## Flujo de ingesta DataOS (contingencia)

`Scheduler` → `IngestService.IngestDataOs` → validar rango → buscar el último estado → `DataProvider.GetMeasurements` → `UnitOfWork.Execute` (guardar el lote y el estado en una TX) → **post-commit**: `EventPublisher.PublishMeasurementsIngested` (solo si `Inserted > 0`; un error se registra como `Warn` y no hace fallar la ingesta).

## DTOs clave

| DTO | Campos |
|---|---|
| `DataRecordCallback` | `event`, `record_id`, `mp_id int64`, `serial_id`, `timestamp string`, `records []CallbackReading` |
| `CallbackReading` | `device_id int64`, `timestamp string`, `name`, `readings Reading` (literal numérico) |
| `CallbackResponse` | `status` (`stored`/`duplicate`), `record_id`, `received_records` |

## Entidades de dominio

- `Device`: `MeasuringPointID`, `Name`, `Active`
- `RawMeasurement`: `Timestamp`, `MeasuringPointID`, `MetricID`, `Value`, `Source`
- `IngestionState`: `MeasuringPointID`, `Source`, `LastIngestedAt`, `ReceivedRecords`, `RecordID *string` (solo para el callback)
- `CallbackRecord` / `CallbackReading`: registro del callback ya validado (timestamps parseados, `RawTimestamp` y `Value` literales)
- `SaveBatchResult`, `SaveStateResult`

## Logs

Logs cortos y en español; el prefijo `[Ingestion]`/`[IngestionCallback]` ya lo pone el logger. El callback **nunca** registra el token ni el cuerpo crudo: un 401 se registra solo con `motivo` y `remote_addr`. Por registro queda una línea `registro guardado` (o `registro duplicado`) y, si aplica, el `WARN` de variables sin mapeo.

## Tablas sin lógica activa

`ingestion_gap_models` (`IngestionGapModel`) y `metric_definitions` (`MetricDefinitionModel`) se siguen migrando para conservar las tablas, pero el módulo ya no tiene entidad, repositorio ni mapper para ellas (se eliminaron el 2026-10-29 por no tener uso). `IngestionUnitOfWork.Execute` entrega solo `MeasurementRepository` e `IngestionStateRepository`.

## Errores (`domain/errors.go`)

`ErrPersistingIngestionState` (`PERSISTING_INGESTION_STATE` → 500), `ErrConsultingDevice` (`CONSULTING_DEVICES` → 500; también lo devuelve `IngestDataOs` si falla la consulta del punto), `ErrConsultingLastState`, `ErrGettingIngestedDataInDataOS`, `ErrRecivingIngestedData` (DataOS sin datos; `cmd/ingestion` lo salta), `NewInvalidCallbackPayloadError(detail)` (`INVALID_INPUT` → 400) y los códigos locales del callback: `UNAUTHORIZED`, `METHOD_NOT_ALLOWED`, `PAYLOAD_TOO_LARGE`, `UNSUPPORTED_MEDIA_TYPE`. El handler responde estos últimos directamente porque `shared/http` no los mapea.

## Eventos publicados

| Tópico | Evento (`contract/`) | Publicado por |
|---|---|---|
| `ingestion.measurements_ingested` | `MeasurementsIngested` | Solo la ingesta DataOS (post-commit, si se insertó al menos 1 medición). El callback ya escribe mediciones pero todavía no lo publica (sin `EventPublisher` cableado). |
