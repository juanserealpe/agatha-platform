# Reglas Fijas y Restricciones (Guardrails)

1. **AISLAMIENTO MODULAR**: Prohibido importar paquetes internos entre módulos (`modules/dashboard` NUNCA importa `modules/ingestion/internal/...`). Comunicación solo vía interfaces públicas/facades en la raíz del módulo o eventos.
2. **CAPA SHARED OBLIGATORIA**: Prohibido `log.Println`, `json.NewEncoder` directo o `fmt.Errorf` genérico. Es obligatorio usar `shared/logger`, `shared/http` y `shared/errors`.
3. **ACCESO A BD COMPARTIDA**: Una sola BD para el monolito. `ingestion` gestiona la ESCRITURA de telemetría; `dashboard` realiza LECTURA (`SELECT`) de métricas y mediciones para visualización.
4. **ERRORES DE DOMINIO**: Envolver siempre con `shared/errors.AppError` antes de llegar al handler HTTP.
5. **CONTEXTO LOCAL Y VERSIÓN**: Todo agente DEBE consultar `backend/internal/modules/[modulo]/README.md` antes de modificar un módulo. Ignorar carpetas/archivos históricos `docs/` salvo instrucción explícita.

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

## Errors (`shared/errors`)

`AppError{Code, Message}` + códigos tipados (`ErrorCode`). Errores de dominio se definen en `domain/errors.go` de cada módulo reutilizando los códigos compartidos.

```go
var ErrCustom = &errors.AppError{Code: errors.CodeNotFound, Message: "recurso no encontrado"}
```

**Códigos disponibles**: `NOT_FOUND`, `INVALID_INPUT`, `FUTURE_DATE`, `ALREADY_INGESTED`, `BEFORE_INITIAL_DATE`, `INVALID_MEASURING_POINT_ID`, `CONSULTING_LAST_STATE`, `GETTING_INGESTED_DATA`.

---

# Modulo: Ingestion

**Responsabilidad**: Ingestar mediciones desde DataOS, procesarlas y persistirlas con control transaccional (Unit of Work). Ejecuta ingestas programadas vía `IngestionScheduler`.

**Arquitectura**: Hexagonal — `application/ports` define interfaces (`DataProvider`, `IngestionUnitOfWork`), `domain/` contiene entidades y repositorios, `infrastructure/` implementa persistencia y cliente externo DataOS.

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

## Flujo de Ingesta

`Scheduler` → `IngestService.IngestDataOs` → validar rango → buscar último estado → `DataProvider.GetMeasurements` → `ProcessingService.Process` → `UnitOfWork.Execute` (save batch + save state en TX).

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

- `DashboardValidator.ValidateDateRange` — `endDate` no puede ser anterior a `startDate`.
- `DashboardValidator.ValidateFutureDate` — `startDate` no puede ser futura.
- Existencia del dispositivo verificada antes de consultar mediciones.





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
- `created_at`, `updated_at` (timestamptz).