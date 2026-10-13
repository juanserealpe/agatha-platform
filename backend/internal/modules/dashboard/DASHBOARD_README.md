# Módulo: Dashboard

## Responsabilidad

Exponer datos de dispositivos y mediciones **ya ingeridos** para consumo del frontend. Solo operaciones de **LECTURA**.

## Estructura Interna

```
dashboard/
├── application/
│   ├── dtos/                  # DTOs de respuesta (DeviceResponse, MeasurementsResponse, etc.)
│   ├── ports/                 # (vacío — sin puertos de aplicación aún)
│   └── services/
│       ├── dashboard_service.go      # Servicio principal: GetAllDevices, GetMeasurements
│       ├── series_service.go         # Variables graficables: GetVariables, GetSeries
│       └── dashboard_validator.go    # Validaciones: rango de fechas, fecha futura
├── domain/
│   ├── errors.go                     # Errores de dominio (AppError) específicos del módulo
│   ├── entities/
│   │   ├── device_summary.go         # DeviceSummary: ID, Name, LastConnection, LastBatteryLevel
│   │   └── measurement.go           # RawMeasurement: MetricID, Timestamp, MetricName, Unit, Value
│   └── repositories/
│       ├── device_repository.go      # DeviceRepository interface
│       └── measurement_repository.go # MeasurementRepository interface
└── infrastructure/
    ├── http/
    │   ├── device_handler.go         # GET dispositivos
    │   └── measurement_handler.go    # GET mediciones (query: deviceId, startDate, endDate)
    └── persistence/
        ├── mappers/                  # Mappers modelo ↔ entidad
        ├── models/                   # Modelos de persistencia (DB)
        └── repositories/            # Implementaciones concretas de repositorios
```

## Endpoints

| Método | Handler | Query Params | Descripción |
|--------|---------|--------------|-------------|
| `GET` | `DeviceHandler.GetAllDevices` | — | Lista dispositivos con última conexión y batería |
| `GET` | `MeasurementHandler.GetMeasurements` | `deviceId`, `startDate` (RFC3339), `endDate` (RFC3339) | Mediciones agrupadas por métrica |
| `GET` | `SeriesHandler.GetVariables` (`/api/dashboard/variables`) | — | Catálogo de variables graficables (`key`, `name`, `unit`, `source`) |
| `GET` | `SeriesHandler.GetSeries` (`/api/dashboard/series`) | `deviceId`, `startDate`, `endDate` (RFC3339), `variables` (opcional: claves separadas por coma; sin él, todas) | Series de tiempo de las variables pedidas, rango `[startDate, endDate)`, en el orden del catálogo |

## Variables graficables (`/series`)

Un solo endpoint devuelve variables de distintas tablas con la misma forma (`timestamp`, `value`). Hoy:

| `key` | Qué es | Fuente | Origen |
|---|---|---|---|
| `b1` | B1 fuga 500-1200Hz | `measurement` | `raw_measurements`, `metric_id = 12` |
| `b2` | B2 fuga 1200-2000Hz | `measurement` | `raw_measurements`, `metric_id = 13` |
| `densidad` | Componente D de la ventana de 24 h (% de registros con B2 sobre el umbral) | `evaluation` | `feature_evaluations.d`, `window_hours = 24` |
| `indicador` | Índice I de la ventana de 24 h (0 a 1) | `evaluation` | `feature_evaluations.i`, `window_hours = 24` |

**Agregar una variable = una línea** en `domain/entities/series_catalog.go`. No se tocan el handler, el servicio ni los lectores:

```go
{Key: "temperatura", Name: "Temperatura", Unit: "°C", Source: SourceMeasurement, MetricID: 20},
{Key: "indicador_6h", Name: "Indicador I (6 h)", Unit: "-", Source: SourceEvaluation, Column: "i", WindowHours: 6},
```

- Fuente `measurement`: cualquier `metric_id` del catálogo de métricas.
- Fuente `evaluation`: cualquier columna de la lista permitida en `evaluation_series_reader.go` (`i`, `coverage`, `d`, `p_hours`, `ex`, `c`, `d_pl`, `z_*`, `s_*`). El punto lleva `evaluated_at` (fin de la ventana). Una hora sin valor (`NO_CALCULABLE`) no devuelve punto: es un hueco en la serie.
- **Agregar una fuente** (otra tabla): implementar `repositories.SeriesReader`, sumar su constante en `entities.SeriesSource` y registrarla en `bootstrap/dashboard.go`.

Los instantes van en UTC. Las series de `evaluation` tienen un punto por hora y las de `measurement` uno por lectura (~15 min).

## DTOs de Respuesta

- **`DeviceResponse`**: `id`, `name`, `lastConnection`, `lastBatteryLevel`
- **`VariableResponse`**: `key`, `name`, `unit`, `source`
- **`SeriesResponse`**: `deviceId`, `startDate`, `endDate`, `series[]`
  - **`VariableSeriesResponse`**: `key`, `name`, `unit`, `source`, `data[]` (`MeasurementPointResponse`; vacío si no hay datos)
- **`MeasurementsResponse`**: `deviceId`, `startDate`, `endDate`, `measurements[]`
  - **`MetricMeasurementsResponse`**: `metricId`, `metricName`, `unit`, `data[]`
    - **`MeasurementPointResponse`**: `timestamp`, `value`

## Contrato de Errores (`domain/errors.go`)

Errores propios del módulo, reutilizando códigos de `shared/errors` (`CodeInvalidInput`, `CodeFutureDate`, `CodeConsultingDevices`, `CodeConsultingMeasurements`). Se usa `appErrors.ErrDeviceNotFound` (definido en `shared/errors`) para el 404.

| Error | Código | HTTP | Cuándo |
|---|---|---|---|
| `ErrMissingDeviceID` | `INVALID_INPUT` | 400 | Falta `deviceId` |
| `ErrInvalidDeviceID` | `INVALID_INPUT` | 400 | `deviceId` no es entero positivo |
| `ErrMissingStartDate` | `INVALID_INPUT` | 400 | Falta `startDate` |
| `ErrInvalidStartDate` | `INVALID_INPUT` | 400 | `startDate` no es RFC3339 |
| `ErrMissingEndDate` | `INVALID_INPUT` | 400 | Falta `endDate` |
| `ErrInvalidEndDate` | `INVALID_INPUT` | 400 | `endDate` no es RFC3339 |
| `ErrInvalidDateRange` | `INVALID_INPUT` | 400 | `startDate >= endDate` |
| `ErrStartDateFuture` | `FUTURE_DATE` | 400 | `startDate` futura |
| `ErrEndDateFuture` | `FUTURE_DATE` | 400 | `endDate` futura |
| `appErrors.ErrDeviceNotFound` | `NOT_FOUND` | 404 | Dispositivo no existe |
| `ErrConsultingDevices` | `CONSULTING_DEVICES` | 500 | Fallo de repositorio en `/devices` |
| `ErrConsultingMeasurements` | `CONSULTING_MEASUREMENTS` | 500 | Fallo de repositorio en `/measurements` (incluye verificación de existencia) |
| `ErrUnknownVariable` | `INVALID_INPUT` | 400 | `variables` trae una clave que no está en el catálogo |
| `ErrConsultingSeries` | `CONSULTING_MEASUREMENTS` | 500 | Fallo de repositorio en `/series` (incluye verificación de existencia) |

Ningún error de GORM/Postgres se expone crudo al cliente: el service siempre envuelve el error del repositorio en uno de los `AppError` anteriores antes de retornarlo al handler.

## Validaciones (`DashboardValidator`)

- `ValidateDateRange(start, end)` — rechaza `start >= end` (incluye igualdad) → `ErrInvalidDateRange`.
- `ValidateNotFuture(date, err)` — genérica, recibe el `AppError` a devolver; se invoca una vez para `startDate` (`ErrStartDateFuture`) y otra para `endDate` (`ErrEndDateFuture`).
- Parseo y validación de formato/positividad de `deviceId`, `startDate`, `endDate` ocurre en el handler (capa HTTP), antes de llegar al service.

## Reglas del Módulo

- Usa SOLO `shared/logger`, `shared/http` y `shared/errors` — nunca `log.Println` ni `json.NewEncoder` directo.
- NO importar nada de `modules/ingestion/...`.
- NO hacer JOINs con tablas de otros módulos. Sí LEE (`SELECT`) `raw_measurements` (Ingestion) y `feature_evaluations` (Processing) con sus propios modelos.
- Errores de dominio se manejan con `*AppError` vía `sharedhttp.HandleError`.

## Estado Actual / Roadmap

- [x] CRUD lectura de dispositivos (`GET /api/dashboard/devices`, retorna `[]` si no hay dispositivos)
- [x] Consulta de mediciones por rango de fechas y dispositivo, rango `[startDate, endDate)`
- [x] Validación estricta de query params (`deviceId` entero positivo, fechas RFC3339 obligatorias)
- [x] Validación de rango de fechas (`startDate < endDate`) y de fecha futura en ambos extremos
- [x] Verificación de existencia de dispositivo antes de consultar (404 si no existe)
- [x] Errores de repositorio (GORM/Postgres) envueltos en `AppError` de dominio antes del handler