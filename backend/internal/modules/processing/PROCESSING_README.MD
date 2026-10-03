# Modulo: Processing

**Responsabilidad por ahora**: Reaccionar a los lotes de mediciones ingeridos por Ingestion. Al recibir el evento `ingestion.measurements_ingested`, **lee de la BD la última ingesta** del punto de medición (la fecha más reciente en `raw_measurements` y todas las variables registradas en esa fecha) y la deja registrada en log. Solo lectura: no escribe en la BD.

## Estructura

```
processing/
├── README.MD
├── domain/
│   ├── errors.go                                  # AppErrors del módulo (reusa códigos de shared/errors)
│   ├── entities/
│   │   ├── metric_definition.go                   # MetricDefinition (vista propia de metric_definitions)
│   │   ├── raw_measurement.go                     # RawMeasurement (+ su MetricDefinition)
│   │   └── latest_ingestion.go                    # LatestIngestion: punto + timestamp + variables
│   └── repositories/
│       └── measurement_repository.go              # Port: GetLatestMeasurements
├── application/
│   └── services/
│       ├── latest_ingestion_service.go            # LatestIngestionService.GetLatestIngestion → log
│       └── latest_ingestion_service_test.go
└── infrastructure/
    ├── events/
    │   ├── measurements_ingested_notice.go        # MeasurementsIngestedNotice (tipo propio de entrada)
    │   └── measurements_ingested_subscriber.go    # Handle → servicio; loguea AppError si falla
    └── persistence/
        ├── models/latest_measurement_model.go     # Fila del JOIN raw_measurements + metric_definitions
        ├── mappers/latest_measurement_mapper.go   # Model → RawMeasurement
        └── repositories/measurement_repository.go # GORM: consulta de la última fecha
```

## Entry Points

| Trigger | Accion | Componente |
|---------|--------|------------|
| Evento `ingestion.measurements_ingested` (post-commit de `IngestService.IngestDataOs`) | Log de recepción → lectura de la última ingesta del punto → log del timestamp, total de variables y cada variable | `MeasurementsIngestedSubscriber.Handle(ctx, MeasurementsIngestedNotice)` → `LatestIngestionService.GetLatestIngestion(ctx, measuringPointID)` |

## Regla de Negocio: Última Ingesta

1. Se valida `measuring_point_id > 0` (si no, `ErrInvalidMeasuringPointID` sin tocar la BD).
2. `MeasurementRepository.GetLatestMeasurements` ejecuta una sola consulta:

```sql
SELECT rm.measuring_point_id, rm.timestamp, rm.value, rm.source, rm.metric_id,
       md.key AS metric_key, md.name AS metric_name, md.unit
FROM raw_measurements AS rm
INNER JOIN metric_definitions AS md ON md.id = rm.metric_id
WHERE rm.measuring_point_id = $1
  AND rm.timestamp = (SELECT MAX(timestamp) FROM raw_measurements WHERE measuring_point_id = $1)
ORDER BY rm.metric_id ASC;
```

3. El resultado es un `LatestIngestion{MeasuringPointID, Timestamp, Measurements}`. El número de variables es `len(Measurements)` (máximo 21, el catálogo fijo).
4. Logs emitidos (`[LatestIngestionService]`): `Última ingesta obtenida` con `measuring_point_id`, `timestamp` (RFC3339), `total_variables`; y una línea `Variable` por medición con `metric_id`, `metric_key`, `value`, `unit`.

## Errores

Definidos en `domain/errors.go`, reutilizando códigos de `shared/errors` (no se agregaron códigos nuevos):

| Error | Código | Cuándo |
|---|---|---|
| `ErrInvalidMeasuringPointID` | `INVALID_MEASURING_POINT_ID` | El evento trae `measuring_point_id <= 0` |
| `ErrConsultingLatestMeasurements` | `CONSULTING_MEASUREMENTS` | Falla la consulta a la BD (el error crudo se loguea en el servicio, nunca se propaga) |
| `ErrLatestMeasurementsNotFound` | `NOT_FOUND` | El punto no tiene mediciones (no debería ocurrir: el evento solo se publica si se insertó ≥ 1) |

Un evento no tiene a quién responderle: el subscriber extrae el `*AppError` con `errors.As` y lo registra con `code` y mensaje. **No hay reintentos**; la próxima ingesta (cada 15 min) vuelve a disparar la lectura.

## Flujo del Evento

`IngestService.IngestDataOs` → commit → `BusEventPublisher` → `shared/eventbus` (cola + worker de la suscripción `processing.measurements_ingested`) → `internal/bootstrap/processing.go` traduce `contract.MeasurementsIngested` → `MeasurementsIngestedNotice` → `MeasurementsIngestedSubscriber.Handle` → `LatestIngestionService.GetLatestIngestion` → `MeasurementRepository.GetLatestMeasurements` (BD) → log.

El `ctx` del handler es el del bus (timeout 30 s por evento), y se propaga hasta la consulta con `WithContext`.

## Reglas de Import (Aislamiento)

- Processing **NO importa `modules/ingestion/...`**, ni siquiera su `contract/` ni sus `entities`. Lo refuerza la regla `processing-no-ingestion` de `depguard` en `backend/.golangci.yml`. Por eso tiene sus propias entidades (`RawMeasurement`, `MetricDefinition`) sobre las mismas tablas, con solo los campos que usa.
- Processing **no importa `shared/eventbus`**: el subscriber es un struct común con `Handle(ctx, Notice)` y no sabe que existe un bus.
- La traducción y la suscripción viven en el composition root: `internal/bootstrap/processing.go` (`registerProcessingSubscribers`). Es el único lugar que conoce ambos módulos.

## Cableado (Composition Root)

Processing se inicializa con `bootstrap.NewProcessingModule(platform)` (`internal/bootstrap/processing.go`), que arma `MeasurementRepository(platform.DB)` → `LatestIngestionService` → `MeasurementsIngestedSubscriber`, registra la suscripción sobre el bus de la `Platform` y devuelve un `ProcessingModule{Unsubscribe}`.

| Entrypoint | Cómo inicializa Processing |
|---|---|
| `cmd/platform` (monolito, imagen Docker) | Explícito en `main`: `NewPlatform` → `NewIngestionModule` → `NewProcessingModule` → `NewDashboardModule` |
| `cmd/scheduler`, `cmd/ingestion` | Vía `bootstrap.NewIngestionApplication()`, que compone Platform + Ingestion + Processing |
| `cmd/api` | No lo inicializa (solo Dashboard; no publica eventos) |

- **Debe registrarse antes de que Ingestion publique**: un evento sin suscriptores se descarta en silencio.
- **Debe vivir en el mismo proceso que Ingestion**: el bus es en memoria. Un contenedor `processing` separado NUNCA recibiría eventos.
- Suscripción: `processing.measurements_ingested` (buffer 256, overflow `DropOldest`, timeout 30 s: los defaults del bus).
- Todos los entrypoints hacen `Platform.Close(ctx)` (timeout 10 s) al terminar, para que el bus drene y el evento llegue a procesarse antes de que salga el proceso.
