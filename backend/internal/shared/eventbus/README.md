# Event Bus (`shared/eventbus`)

Bus de eventos **técnico, en memoria y en proceso** para que los módulos del monolito se comuniquen **sin importarse entre sí**. Solo depende de la stdlib y de `shared/logger`.

> Leé la sección **Reglas** antes de publicar o consumir un evento. Están reforzadas por `depguard` (`backend/.golangci.yml`) y rompen el lint si se violan.

---

## 1. ¿Cuándo usar eventos?

| Necesidad | Mecanismo |
|---|---|
| "Pasó algo en mi módulo y a otros les puede interesar" (notificación asíncrona, el productor no espera respuesta) | **Evento** vía `shared/eventbus` |
| "Necesito un dato de otro módulo ahora" (consulta síncrona) | **Interfaz pública** del módulo dueño (p. ej. `contract.StatusReader`) inyectada desde el composition root |
| "Necesito que otro módulo haga algo y saber si falló" | **No** es un evento: es una llamada a una interfaz pública |

Los eventos son **pistas**, no mensajes garantizados: la base de datos es la fuente de verdad.

---

## 2. Reglas (obligatorias)

1. **El publicador es dueño del contrato.** El tipo del evento y su tópico viven en `modules/<publicador>/contract/`, que **solo importa la stdlib**. Nunca se definen eventos dentro de `shared/eventbus`.
2. **Un consumidor NO importa al publicador**, ni siquiera su `contract/`. Define **su propio tipo** de entrada, y el composition root (`internal/bootstrap`) traduce uno en otro. Es una capa anticorrupción: el publicador puede evolucionar su contrato sin que eso rompa al consumidor.
3. **`domain` y `application` no importan `shared/eventbus`.** Publican a través de un port (`application/ports/event_publisher.go`) implementado en `infrastructure/events/`. Del lado consumidor, el subscriber es un struct común con un método `Handle(ctx, T)`: no conoce el bus.
4. **Solo `internal/bootstrap` (composition root) conoce el bus.** Allí se crea el `InMemoryBus`, se construyen los publishers, se registran las suscripciones y se hace `Close` en el apagado.
5. **Publicar DESPUÉS del commit**, nunca dentro de la transacción. Si el commit falla, no se publica.
6. **Un fallo de publicación NUNCA hace fallar la operación de negocio.** Se registra con `Warn` y se sigue.
7. **Eventos con referencias, no con datos.** Llevan ids, rangos temporales y cursores (`batch_seq`), no las filas. El consumidor lee de la BD lo que necesita.
8. **Consumidores idempotentes.** Un evento puede perderse (overflow, reinicio) o procesarse de nuevo; el consumidor debe tolerar ambos casos y poder reconciliar contra la BD.
9. **Todos los `time.Time` del contrato en UTC**, cada campo con su tag `json` (los eventos deben ser serializables) y un campo `SchemaVersion`.
10. **Handlers cooperativos.** Deben respetar `ctx.Done()`; no bloquear indefinidamente ni hacer trabajo pesado sin cortar por contexto.
11. **Prohibido** `log.Println`, `fmt.Errorf` genérico o `panic` intencional en handlers; usar `shared/logger` y `shared/errors` como en el resto del backend.

### Convenciones de nombres

| Elemento | Convención | Ejemplo |
|---|---|---|
| Tópico | `"<modulo>.<hecho_en_pasado_snake_case>"` | `"ingestion.measurements_ingested"` |
| Constante del tópico | `Topic<Hecho>` en `contract/topics.go` | `TopicMeasurementsIngested` |
| Tipo del evento | Hecho en pasado, PascalCase | `MeasurementsIngested` |
| Tipo propio del consumidor | `<Evento>Notice` | `MeasurementsIngestedNotice` |
| Subscriber | `<Evento>Subscriber` en `infrastructure/events/` del consumidor | `MeasurementsIngestedSubscriber` |
| Nombre de suscripción (`WithName`) | `"<consumidor>.<evento>"` | `"processing.measurements_ingested"` |

### Evolución del contrato

- **Agregar campos** es compatible: mantener `SchemaVersion`.
- **Renombrar, quitar o cambiar la semántica de un campo** es un cambio incompatible: incrementar `SchemaVersion` y actualizar la traducción en `bootstrap` en el mismo PR.

---

## 3. Receta: publicar un evento desde un módulo

Estructura en el módulo publicador `X`:

```
modules/x/
├── contract/
│   ├── topics.go                 # const TopicAlgoOcurrio = "x.algo_ocurrio"; const SchemaVersion = 1
│   └── algo_ocurrio.go           # type AlgoOcurrio struct { ... } (solo stdlib)
├── application/
│   ├── ports/event_publisher.go  # interfaz EventPublisher
│   └── services/...              # publica tras el commit
└── infrastructure/events/
    ├── bus_event_publisher.go    # adaptador sobre eventbus.PublishTyped
    └── noop_event_publisher.go   # para desactivar eventos
```

**1. Contrato** (`contract/algo_ocurrio.go`):

```go
// AlgoOcurrio indica que ... (post-commit). Es una pista; la BD es la fuente de verdad.
type AlgoOcurrio struct {
    EntityID      int64     `json:"entity_id"`
    OccurredAt    time.Time `json:"occurred_at"` // UTC
    SchemaVersion int       `json:"schema_version"`
}
```

**2. Port** (`application/ports/event_publisher.go`):

```go
type EventPublisher interface {
    PublishAlgoOcurrio(ctx context.Context, event contract.AlgoOcurrio) error
}
```

**3. Adaptador** (`infrastructure/events/bus_event_publisher.go`):

```go
func (p *BusEventPublisher) PublishAlgoOcurrio(ctx context.Context, e contract.AlgoOcurrio) error {
    eventbus.PublishTyped(ctx, p.bus, contract.TopicAlgoOcurrio, e)
    return nil
}
```

**4. Servicio**: capturar los resultados dentro de la transacción y publicar **después**:

```go
var saved entities.SaveResult
err := s.unitOfWork.Execute(ctx, func(ctx context.Context, repo repositories.Repo) error {
    r, err := repo.Save(ctx, entity)
    saved = r
    return err
})
if err != nil {
    return err // rollback: NO se publica
}
if s.eventPublisher != nil {
    if err := s.eventPublisher.PublishAlgoOcurrio(ctx, toEvent(saved)); err != nil {
        s.logger.Warn("no se pudo publicar el evento", "error", err) // nunca falla la operación
    }
}
```

## 4. Receta: consumir un evento en otro módulo

En el módulo consumidor `Y` (sin importar a `X`):

```go
// modules/y/infrastructure/events/algo_ocurrio_notice.go
type AlgoOcurrioNotice struct {
    EntityID int64
}

// modules/y/infrastructure/events/algo_ocurrio_subscriber.go
type AlgoOcurrioSubscriber struct { logger *logger.Logger /* + servicios de Y */ }

func (s *AlgoOcurrioSubscriber) Handle(ctx context.Context, n AlgoOcurrioNotice) {
    // idempotente; releer de la BD lo que haga falta
}
```

## 5. Receta: cableado en `internal/bootstrap`

El único lugar que importa `shared/eventbus`, el `contract/` del publicador y el subscriber del consumidor:

```go
// bootstrap/y.go
func registerYSubscribers(bus eventbus.Bus, log *logger.Logger) (unsubscribe func()) {
    subscriber := yEvents.NewAlgoOcurrioSubscriber(log)

    return eventbus.SubscribeTyped(bus, xContract.TopicAlgoOcurrio,
        func(ctx context.Context, e xContract.AlgoOcurrio) {
            subscriber.Handle(ctx, yEvents.AlgoOcurrioNotice{EntityID: e.EntityID}) // traducción
        },
        log,
        eventbus.WithName("y.algo_ocurrio"),
    )
}

// en el constructor de la aplicación
bus := eventbus.NewInMemory(logger.New("EventBus", "Info"))  // tras los pasos que pueden fallar
publisher := xEvents.NewBusEventPublisher(bus)
registerYSubscribers(bus, logger.New("Y", "Info"))

// en main: drenar en el apagado
defer func() {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    _ = app.Close(ctx) // bus.Close
}()
```

Tres detalles que se olvidan seguido:
- **Crear el bus después de los pasos que pueden fallar** (BD, credenciales), para no dejar workers vivos si el arranque aborta.
- **Registrar las suscripciones antes de que empiece a publicarse.** Un evento publicado sin suscriptores se descarta en silencio.
- **Hacer `Close` en el apagado.** Un proceso que termina sin drenar pierde los eventos encolados (p. ej. `cmd/ingestion` corre una vez y sale).

## 6. Checklist de PR

- [ ] El evento y su tópico están en `contract/` del publicador (solo stdlib, tags JSON, UTC, `SchemaVersion`).
- [ ] Se publica después del commit y el error solo se loguea.
- [ ] El consumidor tiene su propio tipo `Notice` y no importa al publicador.
- [ ] La traducción y la suscripción están en `internal/bootstrap`, con `WithName`.
- [ ] El handler es idempotente y respeta `ctx`.
- [ ] Tests: el publicador verifica "commit → publish" y "rollback → no publish"; el cableado tiene un test de punta a punta con el bus real.
- [ ] Se actualizó el **Catálogo de eventos** (abajo) y el README de ambos módulos.
- [ ] `golangci-lint run` (depguard) sin issues.

## 7. Catálogo de eventos

| Tópico | Publicador | Evento (`contract/`) | Consumidores | Suscripción |
|---|---|---|---|---|
| `ingestion.measurements_ingested` | Ingestion (`IngestService.IngestDataOs`, post-commit, si `Inserted > 0`) | `MeasurementsIngested` | Ninguno cableado hoy | — |
| `processing.indicator_evaluated` | Processing (`EvaluationService.EvaluatePending`, post-commit, uno por ventana 24 h/6 h, solo si el lote se insertó completo; no en `Backfill`) | `IndicatorEvaluated` | Rule Engine (`EvaluateIndicator`) | `rule_engine.indicator_evaluated` |
| `rule_engine.level_evaluated` | Rule Engine (`EvaluationService`, post-commit, cada hora de 24 h `CALCULATED` —incluida NORMAL— y `NO_CALCULABLE` sin fila; no en backfill, duplicados ni llegadas tardías) | `LevelEvaluated` (`SchemaVersion = 2`) | Ninguno hoy (Alert Management) | — |

---

## 8. Contrato del bus

```go
type Bus interface {
    Publish(ctx context.Context, topic string, event any)            // no bloqueante, no falla
    Subscribe(topic string, h Handler, opts ...SubscribeOption) (unsubscribe func())
    Stats() []SubscriptionStats
    Close(ctx context.Context) error                                 // drena hasta el deadline de ctx
}
```

Implementación: `NewInMemory(log, opts...)`. Helpers tipados: `PublishTyped[T]` y `SubscribeTyped[T]` (este último descarta y loguea como error un evento que no sea `T`, sin entrar en pánico). Preferí siempre los helpers tipados antes que `Publish`/`Subscribe` con `any`.

## 9. Garantías

| Garantía | Detalle |
|---|---|
| Publicación no bloqueante | `Publish` nunca bloquea ni devuelve error. Sin suscriptores es un no-op silencioso. |
| Orden FIFO por suscriptor | Cada suscripción tiene su propia cola acotada y **una** goroutine worker. Entre suscriptores no hay orden relativo. |
| Aislamiento | Un suscriptor lento o que entra en pánico no afecta a los demás ni al productor. |
| Contexto desconectado | El handler recibe `context.WithoutCancel(ctxPublicador)`: conserva los valores (trace, request id) pero NO la cancelación. Expira por `handlerTimeout`, por `unsubscribe` o por el deadline de `Close`. |
| Pánicos | Se recuperan con `recover()`, se loguean con tópico y stack, incrementan `Panics` y el worker continúa. |
| `unsubscribe` | Idempotente y seguro con concurrencia. Descarta lo pendiente (cuenta como `Dropped`) y cancela el ctx del handler en curso. Puede llamarse desde dentro de un handler. |
| `Close` | Deja de aceptar eventos y suscripciones, drena las colas y espera a los workers. Si `ctx` expira, descarta lo pendiente, cancela el ctx de los handlers en curso y devuelve `ctx.Err()`. Idempotente. |

**Semántica best-effort**: un evento puede perderse (overflow, cierre, caída del proceso). No hay persistencia, reintentos ni entrega entre procesos.

## 10. Overflow

Cuando la cola de una suscripción está llena:

| Política | Efecto |
|---|---|
| `DropOldest` (default) | Descarta el evento más antiguo de la cola y encola el nuevo. |
| `DropNewest` | Descarta el evento entrante. |

Cada descarte incrementa `Dropped`. El warn se registra como máximo **una vez cada 5 s por suscripción** (lo mismo aplica a publicar tras `Close`).

## 11. Configuración

| Opción | Default | Nota |
|---|---|---|
| `WithDefaultBuffer(n)` (bus) | `256` | Capacidad de cola por suscripción. |
| `WithDefaultHandlerTimeout(d)` (bus) | `30s` | Timeout del ctx del handler. |
| `WithName(s)` | `"<topic>#<id>"` | Aparece en logs y `Stats`. Siempre asignarlo (ver convenciones). |
| `WithBuffer(n)` / `WithOverflow(p)` / `WithHandlerTimeout(d)` | los del bus | Por suscripción. |

Los valores inválidos (`<= 0`, política desconocida, nombre vacío) se ignoran y se conserva el default. Hoy el bus se crea con los defaults en `internal/bootstrap/ingestion.go`; aún no hay variables de entorno `EVENTBUS_*`.

## 12. Observabilidad (`Stats`)

`SubscriptionStats{Name, Topic, Published, Delivered, Dropped, Panics, QueueLen}`, ordenadas por tópico y nombre. Invariante tras drenar: `Published = Delivered + Panics + Dropped`.

`Delivered` cuenta ejecuciones del handler que terminaron sin pánico. Un evento descartado por `SubscribeTyped` por tipo incorrecto cuenta como `Delivered` (el handler envoltorio terminó normalmente); el descarte queda en el log de error.

## 13. Límites (leer antes de usar)

- **Timeout cooperativo**: Go no puede matar goroutines. Un handler que ignora `ctx.Done()` sigue ocupando su worker hasta terminar; `Close` devuelve `ctx.Err()` a tiempo, pero esa goroutine vive hasta que el handler retorne.
- **Solo en proceso**: sin durabilidad. Un reinicio pierde lo encolado. Si algún flujo necesita garantía de entrega, eso es un outbox en BD, no este bus.
- **Un worker por suscripción**: un handler lento retrasa solo su propia cola. Si necesitás paralelismo, hacelo dentro del consumidor, no con múltiples suscripciones al mismo tópico para el mismo trabajo.
