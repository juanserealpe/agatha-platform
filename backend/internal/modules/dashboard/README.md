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

## DTOs de Respuesta

- **`DeviceResponse`**: `id`, `name`, `lastConnection`, `lastBatteryLevel`
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

Ningún error de GORM/Postgres se expone crudo al cliente: el service siempre envuelve el error del repositorio en uno de los `AppError` anteriores antes de retornarlo al handler.

## Validaciones (`DashboardValidator`)

- `ValidateDateRange(start, end)` — rechaza `start >= end` (incluye igualdad) → `ErrInvalidDateRange`.
- `ValidateNotFuture(date, err)` — genérica, recibe el `AppError` a devolver; se invoca una vez para `startDate` (`ErrStartDateFuture`) y otra para `endDate` (`ErrEndDateFuture`).
- Parseo y validación de formato/positividad de `deviceId`, `startDate`, `endDate` ocurre en el handler (capa HTTP), antes de llegar al service.

## Reglas del Módulo

- Usa SOLO `shared/logger`, `shared/http` y `shared/errors` — nunca `log.Println` ni `json.NewEncoder` directo.
- NO importar nada de `modules/ingestion/...`.
- NO hacer JOINs con tablas de otros módulos.
- Errores de dominio se manejan con `*AppError` vía `sharedhttp.HandleError`.

## Estado Actual / Roadmap

- [x] CRUD lectura de dispositivos (`GET /api/dashboard/devices`, retorna `[]` si no hay dispositivos)
- [x] Consulta de mediciones por rango de fechas y dispositivo, rango `[startDate, endDate)`
- [x] Validación estricta de query params (`deviceId` entero positivo, fechas RFC3339 obligatorias)
- [x] Validación de rango de fechas (`startDate < endDate`) y de fecha futura en ambos extremos
- [x] Verificación de existencia de dispositivo antes de consultar (404 si no existe)
- [x] Errores de repositorio (GORM/Postgres) envueltos en `AppError` de dominio antes del handler