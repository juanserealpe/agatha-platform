# DataOS Adapter

Adapter de **contingencia** del módulo Ingestion: implementa `ports.DataProvider` consultando DataOS. La ingesta principal es el callback de Dataflow Labs; este adapter se mantiene para el scheduler (`AGATHA_MODULES=dataos`, `cmd/scheduler`) y la carga histórica (`cmd/ingestion`).

El resto del módulo no conoce endpoints, cookies ni JSON de DataOS.

## Estructura

```
dataos/
├── client.go               # Client.GetMeasurements: login → render → extraer → mapear
├── auth/                   # POST /api/auth/login (sesión por cookie)
├── dashboard/              # POST /api/v2/dashboard/configurations/10/render y sus modelos
└── mapping/
    ├── series_extractor.go # componentes del dashboard → series
    ├── mapper.go           # serie → RawMeasurement (catálogo de 21 métricas)
    └── timestamp_parser.go # RFC 3339 → America/Bogota
```

Las series fuera del catálogo se omiten y se informan en un único `WARN` (`series sin mapeo; no se guardan`).
