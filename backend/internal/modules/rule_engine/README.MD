# Modulo: Rule Engine

**Responsabilidad**: interpretar las evaluaciones de Processing con reglas (umbrales, rachas, categorías) para decidir alertas. Processing **mide**; el Rule Engine **interpreta**.

**Estado actual**: solo **recibe** el evento `processing.indicator_evaluated` y lo registra en el log. Todavía no tiene reglas, persistencia ni endpoints.

**Próximo paso previsto**: una regla de **umbral del indicador I**, editable con un `POST` (aún no implementada).

## Estructura

```
rule_engine/
└── infrastructure/
    └── events/
        ├── indicator_evaluated_notice.go      # IndicatorEvaluatedNotice: vista propia del evento
        └── indicator_evaluated_subscriber.go  # IndicatorEvaluatedSubscriber.Handle → logger.Info
```

## Eventos Consumidos

| Tópico | Publicador | Notice propio | Suscripción (`WithName`) | Acción |
|---|---|---|---|---|
| `processing.indicator_evaluated` | Processing (`EvaluatePending`, post-commit) | `IndicatorEvaluatedNotice` | `rule_engine.indicator_evaluated` | Log `Info` |

Llega **un evento por ventana**: cada hora evaluada produce uno de 24 h y otro de 6 h, incluidos los `NO_CALCULABLE`. El histórico (`processing -backfill`) **no** genera eventos.

`IndicatorEvaluatedNotice`: `MeasuringPointID`, `WindowHours` (24 | 6), `EvaluatedAt` (fin exclusivo de la ventana, UTC), `Status` (`CALCULATED` | `NO_CALCULABLE`), `NoCalcReason`, `I` (`*float64`, nil si no se calculó), `LowConfidence`, `Trigger`, `OccurredAt`.

Ejemplo de log:

```
[2026-10-15 07:35:02 -05:00] [INFO] [RuleEngine] evento de indicador evaluado recibido [measuring_point_id 11556 window_hours 24 evaluated_at 2026-10-15T07:00:00-05:00 status CALCULATED i 0.12 low_confidence false trigger RECONCILER]
```

**Los eventos son pistas** (best-effort, en memoria): pueden perderse (overflow, reinicio) o llegar dos veces. Las reglas que se agreguen deben ser idempotentes y poder reconciliar contra `feature_evaluations`, que es la fuente de verdad.

## Reglas de Import (Aislamiento)

No importa `modules/processing` (ni su `contract/`), `modules/ingestion`, `modules/baseline` ni `shared/eventbus` (`depguard` `rule-engine-isolation`). El subscriber es un struct común con `Handle(ctx, Notice)`: no conoce el bus.

## Cableado (Composition Root)

`bootstrap.NewRuleEngineModule(platform)` (`internal/bootstrap/rule_engine.go`) suscribe el subscriber al bus y traduce `processing/contract.IndicatorEvaluated` → `IndicatorEvaluatedNotice`. Se llama dentro de `NewApplication`, antes de arrancar los schedulers, para no perder eventos.

| Entrypoint | Rule Engine |
|---|---|
| `cmd/platform` | Módulo `rule_engine` de `AGATHA_MODULES`. **Requiere `processing`** (sin él no llega ningún evento). No está en el default |

## Tests

- `infrastructure/events/indicator_evaluated_subscriber_test.go`: el log incluye punto, ventana, estado, I y trigger; un `NO_CALCULABLE` registra el motivo sin imprimir punteros.
- `internal/bootstrap/rule_engine_test.go`: de punta a punta con el bus real (publica el contrato de Processing → llega el Notice traducido, suscripción `rule_engine.indicator_evaluated`).
