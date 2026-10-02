# Modulo: Processing

**Responsabilidad**: Reaccionar a los lotes de mediciones ingeridos por Ingestion. Hoy solo **recibe** el evento `ingestion.measurements_ingested` y registra su recepción en log (observabilidad del flujo entre módulos). No lee ni escribe en la BD.

**Arquitectura**: Hexagonal. Por ahora solo existe la capa `infrastructure/events` (adaptador de eventos entrantes); `domain/` y `application/` están vacíos.

## Estructura

```
processing/
├── README.MD
├── domain/                                        # vacío
├── application/                                   # vacío
└── infrastructure/
    └── events/
        ├── measurements_ingested_notice.go        # MeasurementsIngestedNotice (tipo propio de entrada)
        └── measurements_ingested_subscriber.go    # MeasurementsIngestedSubscriber.Handle → log
```

## Entry Points

| Trigger | Accion | Componente |
|---------|--------|------------|
| Evento `ingestion.measurements_ingested` (post-commit de `IngestService.IngestDataOs`) | `logger.Info("evento de mediciones ingeridas recibido", ...)` | `MeasurementsIngestedSubscriber.Handle(ctx, MeasurementsIngestedNotice)` |

## Tipos Clave

| Tipo | Campos |
|------|--------|
| `MeasurementsIngestedNotice` | `MeasuringPointID int64`, `Source string`, `From time.Time`, `To time.Time`, `Watermark time.Time`, `BatchSeq int64`, `InsertedRecords int` |

Es una **pista**: indica que hay mediciones nuevas para `MeasuringPointID` entre `From` y `To` (UTC). La fuente de verdad son las mediciones en la BD. No trae `OccurredAt` ni `SchemaVersion` del contrato de Ingestion porque Processing no los usa; si hicieran falta, se agregan acá y en la traducción.

## Flujo del Evento

`IngestService.IngestDataOs` → commit → `BusEventPublisher` → `shared/eventbus` (cola + worker de la suscripción `processing.measurements_ingested`) → `internal/bootstrap/processing.go` traduce `contract.MeasurementsIngested` → `MeasurementsIngestedNotice` → `MeasurementsIngestedSubscriber.Handle` → log con `measuring_point_id`, `source`, `batch_seq`, `inserted_records`, `from`, `to`, `watermark`.

Ejemplo de salida:

```
[2026-09-29 21:40:00] [INFO] [Processing] evento de mediciones ingeridas recibido [measuring_point_id 11556 source dataos batch_seq 42 inserted_records 3 from ... to ... watermark ...]
```

## Reglas de Import (Aislamiento)

- Processing **NO importa `modules/ingestion/...`**, ni siquiera su `contract/`. Lo refuerza la regla `processing-no-ingestion` de `depguard` en `backend/.golangci.yml`.
- Processing **no importa `shared/eventbus`**: el subscriber es un struct común con `Handle(ctx, Notice)` y no sabe que existe un bus.
- La traducción y la suscripción viven en el composition root: `internal/bootstrap/processing.go` (`registerProcessingSubscribers`). Es el único lugar que conoce ambos módulos.

## Cableado (Composition Root)

Processing se inicializa con `bootstrap.NewProcessingModule(platform)` (`internal/bootstrap/processing.go`), que registra la suscripción sobre el bus de la `Platform` y devuelve un `ProcessingModule{Unsubscribe}`.

| Entrypoint | Cómo inicializa Processing |
|---|---|
| `cmd/platform` (monolito, imagen Docker) | Explícito en `main`: `NewPlatform` → `NewIngestionModule` → `NewProcessingModule` → `NewDashboardModule` |
| `cmd/scheduler`, `cmd/ingestion` | Vía `bootstrap.NewIngestionApplication()`, que compone Platform + Ingestion + Processing |
| `cmd/api` | No lo inicializa (solo Dashboard; no publica eventos) |

- **Debe registrarse antes de que Ingestion publique**: un evento sin suscriptores se descarta en silencio.
- **Debe vivir en el mismo proceso que Ingestion**: el bus es en memoria. Un contenedor `processing` separado NUNCA recibiría eventos.
- Suscripción: `processing.measurements_ingested` (buffer 256, overflow `DropOldest`, timeout 30 s: los defaults del bus).
- Todos los entrypoints hacen `Platform.Close(ctx)` (timeout 10 s) al terminar, para que el bus drene y el evento llegue a loguearse antes de que salga el proceso.

## Garantías y Límites

- Best-effort: un evento puede perderse (overflow, reinicio). Cuando Processing haga trabajo real, deberá ser **idempotente** y poder reconciliar contra la BD usando `BatchSeq` y `Watermark`.
- El handler recibe un ctx que conserva los valores del publicador pero no su cancelación, con timeout de 30 s.
- Reglas completas de comunicación entre módulos: `internal/shared/eventbus/README.md`.

## Cómo Extender

1. Agregar el caso de uso en `application/services` (y sus ports) y llamarlo desde `MeasurementsIngestedSubscriber.Handle` en lugar de solo loguear.
2. Si hacen falta más datos del evento, agregarlos a `MeasurementsIngestedNotice` y a `toMeasurementsIngestedNotice` en `internal/bootstrap/processing.go`.
3. Si Processing necesita publicar sus propios eventos, seguir la receta "publicar" de `internal/shared/eventbus/README.md` (`contract/` propio + port + `infrastructure/events`).
