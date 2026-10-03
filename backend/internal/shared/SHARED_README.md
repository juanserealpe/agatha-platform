# Capa Shared (`internal/shared`)

Utilidades reutilizables de consumo **OBLIGATORIO**. No reinventar — usar lo que ya existe.

## 1. Errores de Dominio (`shared/errors`)

### Códigos (`ErrorCode`)

| Constante | Valor | HTTP Status |
|---|---|---|
| `CodeNotFound` | `NOT_FOUND` | 404 |
| `CodeInvalidInput` | `INVALID_INPUT` | 400 |
| `CodeFutureDate` | `FUTURE_DATE` | 400 |
| `CodeAlreadyIngested` | `ALREADY_INGESTED` | 409 |
| `CodeBeforeInitialDate` | `BEFORE_INITIAL_DATE` | 400 |
| `CodeInvalidMeasuringPointID` | `INVALID_MEASURING_POINT_ID` | 400 |
| `CodeConsultingLastState` | `CONSULTING_LAST_STATE` | 500 |
| `CodeGettingIngestedData` | `GETTING_INGESTED_DATA` | 502 |
| `CodeConsultingDevices` | `CONSULTING_DEVICES` | 500 |
| `CodeConsultingMeasurements` | `CONSULTING_MEASUREMENTS` | 500 |
| `CodeConsultingBaseline` | `CONSULTING_BASELINE` | 500 |
| `CodePersistingBaseline` | `PERSISTING_BASELINE` | 500 |

### Errores pre-construidos

`ErrNotFound`, `ErrDeviceNotFound`, `ErrFutureDate`, `ErrInvalidDate`

### Uso

```go
// Retornar error existente
return sharedErr.ErrNotFound

// Crear error de dominio nuevo
return &sharedErr.AppError{Code: sharedErr.CodeInvalidInput, Message: "campo X requerido"}
```

## 2. Helpers HTTP (`shared/http`)

| Función | Firma |
|---|---|
| `WriteJSON` | `WriteJSON(w http.ResponseWriter, status int, data any)` |
| `HandleError` | `HandleError(w http.ResponseWriter, err error)` |

**PROHIBIDO** usar `json.NewEncoder` o escribir headers manualmente. Usar siempre `WriteJSON`.

```go
// Respuesta exitosa
sharedHTTP.WriteJSON(w, http.StatusOK, map[string]any{"items": results})

// Respuesta de error (mapea AppError → HTTP status automáticamente)
sharedHTTP.HandleError(w, err)
```

## 3. Logger (`shared/logger`)

Métodos: `Info(msg, kv...)`, `Warn(msg, kv...)`, `Error(msg, kv...)`. Timezone fijo: `America/Bogota`.

```go
log := logger.New("ingestion", "INFO")
log.Error("falló extracción", "deviceID", id, "err", err)
```

## 4. Event Bus (`shared/eventbus`)

Bus de eventos en memoria para comunicar módulos **sin importarse entre sí**. `Publish` nunca bloquea ni falla; cada suscripción tiene su cola acotada y un worker FIFO. Semántica best-effort: los eventos son **pistas**, la BD es la fuente de verdad.

| Elemento | Firma / Uso |
|---|---|
| Crear | `eventbus.NewInMemory(log, opts...)` — solo en `internal/bootstrap` |
| Publicar | `eventbus.PublishTyped(ctx, bus, topic, event)` |
| Suscribir | `eventbus.SubscribeTyped(bus, topic, handler, log, eventbus.WithName("<consumidor>.<evento>"))` |
| Cerrar | `bus.Close(ctx)` — drena hasta el deadline de `ctx` |

**Reglas clave** (detalle, recetas y catálogo de eventos en [`eventbus/README.md`](eventbus/README.md)):

- El evento y su tópico viven en `modules/<publicador>/contract/` (solo stdlib).
- El consumidor **no** importa al publicador: define su propio tipo y `internal/bootstrap` traduce.
- `domain`/`application` no importan `shared/eventbus`; se publica vía port `EventPublisher`.
- Publicar **después del commit**; un error de publicación se loguea y nunca falla la operación.

```go
// internal/bootstrap
bus := eventbus.NewInMemory(logger.New("EventBus", "Info"))
publisher := events.NewBusEventPublisher(bus) // adaptador del módulo publicador
registerProcessingSubscribers(bus, logger.New("Processing", "Info"))
defer bus.Close(shutdownCtx)
```

## 5. Kernel de cálculo (`shared/features`)

Fórmulas de la línea base y del índice I (`Normalize`, `Quantile`, `Median`, `MAD`, `Spearman`, `ComputeComponents`, `MADFloor`, `Score`). Lo usa Baseline y lo reutilizará Processing **sin reimplementar fórmulas**. Solo stdlib, sin IO ni estado; versionado con `AlgorithmVersion`. Detalle en [`features/README.md`](features/README.md).
