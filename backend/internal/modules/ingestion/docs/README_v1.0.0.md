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

## Reglas del Módulo

- Usa SOLO `shared/logger`, `shared/http` y `shared/errors` — nunca `log.Println` ni `json.NewEncoder` directo.
- NO importar nada de `modules/ingestion/...`.
- NO hacer JOINs con tablas de otros módulos.
- Errores de dominio se manejan con `*AppError` vía `sharedhttp.HandleError`.

## Estado Actual / Roadmap

- [x] CRUD lectura de dispositivos
- [x] Consulta de mediciones por rango de fechas y dispositivo
- [x] Validación de fechas (rango, fecha futura)
- [x] Verificación de existencia de dispositivo antes de consultar