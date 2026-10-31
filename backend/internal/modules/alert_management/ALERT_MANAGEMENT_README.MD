# Modulo: Alert Management

**Responsabilidad**: convertir el nivel horario que publica el Rule Engine (`rule_engine.level_evaluated` v3) en **alertas** para el operador (web) y el usuario de campo (app móvil). Crea una alerta por dispositivo y evento y mantiene su nivel actual y su nivel máximo. Registra la línea de tiempo (colores y mensajes), decide qué mensajes son **notificaciones** (alertas nuevas y subidas de nivel) y guarda la gestión del operador: clasificación, etiqueta y observaciones.

**No entrega avisos**: no hay push, FCM, web push ni registro de celulares. Cada mensaje `NOTIFICACION` se publica como evento `alert_management.alert_notification` y el canal (web, push, correo) y los reintentos son del módulo de notificaciones.

**No recalcula niveles ni cambia umbrales**: los niveles son del Rule Engine y los umbrales se editan en `POST /api/rule-engine/points/{id}/thresholds`.

Especificación vigente: `backend/AGATHA_Modulo_Alert_Management_v3.3.docx` (Backlog 1.5). Las fotos (RN-09, HE-06, D-37) quedan fuera de esta entrega. Todo se guarda en **UTC** y la API responde en **hora de Colombia** con offset (RN-11).

## Estructura

```
alert_management/
├── contract/                 # TopicAlertNotification + AlertNotification, TopicAlertChanged + AlertChanged (solo stdlib; bootstrap lo traduce para Notification)
├── domain/
│   ├── errors.go             # AppError del módulo
│   ├── entities/             # Alert, Management, Runtime, Segment, Message, Observation, ProcessingState, Policy, LevelNotice, enums
│   ├── messages/catalog.go   # catálogo estático: código → título/cuerpo + MessageCatalogVersion
│   └── services/decide.go    # Decide (PURO): transiciones del ciclo + reglas de mensaje en orden
├── application/
│   ├── ports/                # AlertStore/AlertTx, PolicyStore, LevelSource, DeviceDirectory, AlertQueries, NotificationPublisher, SpreadsheetWriter
│   └── services/             # ProcessingService, ReplayService (catch-up + backfill), ManagementService, QueryService (+ export), PolicyService
└── infrastructure/
    ├── events/               # LevelEvaluatedSubscriber → ProcessingService (LIVE)
    ├── http/                 # AlertHandler (/api/alerts) + DTOs
    ├── scheduling/           # CatchUpScheduler (minuto 55)
    ├── export/               # XLSXWriter (stdlib: zip + SpreadsheetML)
    └── persistence/          # schema.go (Migrate + seeds), models/, mappers/, repositories/ (GORM), memory/ (pruebas)
```

## Ciclo de vida (sección 3)

| Estado | Entra | Sale |
|---|---|---|
| Normal (sin alerta) | — | `episode_event = OPENED` ⇒ alerta nueva (OPEN, notifica) o reactivación de la última |
| ACTIVA | Se crea o se reactiva | `episode_event = CLEARED` ⇒ FINALIZADA. Clasificar o etiquetar **no** cambia el estado |
| FINALIZADA | Volvió a Normal | `OPENED` antes de `merge_window_hours` (24) y Sin clasificar ⇒ ACTIVA (misma alerta). Siempre se puede gestionar |

- **Fecha inicio** (`started_at`): primera hora sobre el umbral de la racha que abrió el episodio. Se toma de `alert_processing_state.pending_run_started_at`; si la racha no se vio, se deduce de `hours_above`. `opened_at` es la hora en que se cumple la racha y se crea la alerta. **Fecha fin**: la hora del `CLEARED`.
- **Una ACTIVA por (punto, evento)**: índice único parcial `ux_alerts_one_active`.
- **Sin datos** (`NO_CALCULABLE` en vivo o un salto de `evaluated_at` en el catch-up y el backfill): la alerta sigue activa y conserva su nivel. Las cuentas se pausan y se suma `hours_no_data`.
- **Reactivación**: abre un color nuevo con el nivel que tenía al finalizar, o el nuevo si es mayor. Notifica solo si supera el máximo. Si la anterior ya estaba clasificada ⇒ alerta nueva con `NEW_AFTER_CLASSIFIED`. Si pasaron 24 h o más ⇒ alerta nueva con `OPEN`. Las horas de la pausa no suman a la duración.
- **Sube**: `computed_level` > nivel actual ⇒ color nuevo + notificación. Con la etiqueta `MANTENIMIENTO_PROGRAMADO`, las subidas hasta `intervention_silent_max_level` (NARANJA) no notifican (`UP_DURING_INTERVENTION`); el rojo **siempre** notifica.
- **Baja un nivel**: `downgrade_hours` (6) horas **calculadas** seguidas con `computed_level` por debajo del actual ⇒ color nuevo, sin notificar. Nunca baja de AMARILLO: termina con el `CLEARED`.
- **Rojo sin clasificar** (`rojo_requires_classification_to_downgrade`): no baja y titila. Emite **un** `ROJO_HELD`. Al clasificarla, la hora siguiente baja si ya cumplió las 6 h.
- **Mapa**: `blinking = state ACTIVA ∧ current_level = ROJO ∧ Sin clasificar`. Con varias alertas activas en un punto, el color es el de mayor criticidad (`GET /active`).

## Mensajes (sección 6)

`Decide` devuelve código y variables, y el texto sale de `domain/messages/catalog.go`. Cada mensaje guarda el texto renderizado, el I de la hora, `params`, `catalog_version` y `policy_version_id`, dentro del color vigente en su hora (o del color nuevo, si la hora lo abrió). Con `low_confidence` se agrega "Sensor en ajuste: resultado preliminar." `UNIQUE (alert_id, evaluated_at, code)`: reprocesar no duplica.

| Orden | Códigos | Cuándo |
|---|---|---|
| 1 (siempre) | `OPEN` (variante por nivel), `UP_NARANJA_PERSIST` / `UP_NARANJA_FAST` / `UP_ROJO` (variante "sostenido N h"), `REACTIVATED`, `NEW_AFTER_CLASSIFIED`, `UP_DURING_INTERVENTION` | Crear, subir, reactivar. "Por duración" = motivo `LEAK_PERSISTENCE` en la hora |
| 2 | `NO_CALC_START` / `NO_CALC_END` | Empieza o termina una racha sin cálculo |
| 3 | `DOWNGRADE` / `ROJO_HELD` | Baja tras 6 h / el rojo sin clasificar no baja (una vez) |
| 4 | `OSCILLATING` | `oscillation_flips` (4) cambios entre "sobre el umbral" y "banda de cierre" en `oscillation_window_hours` (6). Silencia 5 y 6 hasta 6 h sin cambios |
| 5 | `ALMOST_NORMAL` / `ONE_HOUR_NORMAL` / `BACK_ABNORMAL` | Entra a la banda (raw NORMAL, I ≥ salida) / 1 h bajo la salida (`below_run = 1`) / vuelve sobre el umbral |
| 6 | `DOWNGRADE_START` | Primera hora por debajo del nivel actual (solo NARANJA o ROJO) |
| 7 | `COUNTDOWN` | `escalation − runs = countdown_hours` (3), una vez por nivel |
| 8 | `TREND` | Cambia la tendencia: I frente a `trend_lookback_hours` (3) antes, diferencia redondeada a 2 decimales, ±`trend_delta` (0,02) |
| 9 | `SUMMARY` | `summary_every_hours` (6) sin mensajes |
| fin | `END_NORMAL` | `CLEARED` |
| gestión | `CLASSIFIED` / `LABELED` | El operador clasifica o etiqueta (kind `GESTION`) |

En cada hora se emite siempre la regla 1 y, de las demás, gana **la primera que aplica**. Tras una subida, solo se evalúan las reglas 2 a 4 (la notificación ya dice qué cambió). Una prueba falla si un texto usa una variable que su código no declara. El lenguaje es de campo: nunca "I", "umbral" ni "persistencia".

## Gestión (sección 4)

- **Clasificar** (`CONFIRMADA` | `FALSA_ALERTA`): solo Sin clasificar. Se escribe con `UPDATE … WHERE classification IS NULL`; si no afecta filas, es un **conflicto (RN-18)**. Prevalece la primera, la segunda se guarda como observación `CONFLICTO_CLASIFICACION` y la respuesta es 409 con la clasificación vigente.
- **Etiquetar**: solo Clasificada sin etiqueta y con la lista de su clasificación. Sin clasificar ⇒ 409; otra lista ⇒ 400. `OTRO` exige `other_text` de 1 a 100 caracteres; con otra etiqueta, el texto no se envía (400). Una segunda etiqueta ⇒ conflicto `CONFLICTO_ETIQUETA`.
- **Observaciones**: de 1 a 500 caracteres, solo se agregan. Se pueden enviar con la clasificación o la etiqueta, o solas.
- **Origen (RN-10)**: `origin` `WEB` | `APP_MOVIL`. Desde la app, `device_identifier` es obligatorio. `client_record_id` (UUID) es opcional: reenviarlo ⇒ 200 `ALREADY_RECEIVED`, sin duplicados. `recorded_at` es la hora en el celular.
- **Inmutable desde la BD**: el trigger `trg_alerts_management_immutable` rechaza cambiar la clasificación o la etiqueta y todo `DELETE`, salvo `-rebuild` con `SET LOCAL agatha.alerts_rebuild = 'on'`. El procesamiento horario solo escribe sus columnas (`mappers.ProcessingColumns`).

| Clasificación | Etiquetas | ¿Se excluye de la línea base? |
|---|---|---|
| CONFIRMADA | `FUGA_CONFIRMADA_CAMPO`, `MOVIMIENTO_TIERRA`, `MAQUINARIA_AJENA`, `OTRO` | Sí (también sin etiqueta) |
| FALSA_ALERTA | `CLIMA_EXTREMO`, `TRANSITO_PESADO`, `OTRO` | Sí |
| FALSA_ALERTA | `MANTENIMIENTO_PROGRAMADO` | Sí |

Decisión (2026-10-26, cambia la propuesta técnica del documento, que excluía solo Confirmadas y mantenimiento): **toda alerta se excluye de la línea base**, cualquiera sea su clasificación, incluidas Sin clasificar y activas. La regla vive en Baseline (`AlertEventsReader`).

## Notificaciones (sección 5): eventos para el módulo de notificaciones

Alert Management decide **qué** es una notificación (`kind = NOTIFICACION` en el catálogo y las reglas de `Decide`: apertura, subidas, alerta nueva tras clasificada; las subidas silenciosas por intervención no lo son). **Cómo** se entrega no es asunto de este módulo.

- **Evento**: `alert_management.alert_notification` (`contract.AlertNotification`, `schema_version` 1), uno por cada mensaje `NOTIFICACION` **nuevo**. Lleva `message_id` (clave de idempotencia), `alert_id`, `alert_code`, `measuring_point_id`, `event_type`, `started_at`, `evaluated_at`, `code`, `level`, `max_level`, `alert_state`, `classification` (vacío = sin clasificar; el consumidor omite el push de `FALSA_ALERTA`), `title` y `body` ya renderizados, `indicator_value`, `source` (`LIVE` | `CATCH_UP`), `late`, `policy_version_id` y `occurred_at`.
- **Cuándo**: después del commit de la hora. Si la TX falla no se publica nada; si falla la publicación, la alerta ya quedó guardada (se loguea). En el backfill y en reprocesos (el mensaje ya existía) no se publica.
- **`late`**: la hora llegó con más de `late_notification_max_hours` (2) de atraso (catch-up, Processing reponiéndose de una caída). La especificación pide no enviar push en ese caso; lo aplica el consumidor.
- **Best-effort**: el bus es en memoria (un evento se pierde si el proceso cae entre el commit y la publicación). La fuente de verdad es `alert_messages` (`kind = 'NOTIFICACION'`, índice `ix_alert_messages_kind`): el módulo de notificaciones puede reconciliar por `id`.
- El adaptador del puerto `NotificationPublisher` al bus es `busAlertNotificationPublisher` (`internal/bootstrap`); sin suscriptores, publicar es un no-op.

### Cambios visibles: `alert_management.alert_changed`

Para que el mapa y el panel se actualicen en tiempo real sin consultar la API.

- **Evento**: `contract.AlertChanged` (`schema_version` 1), una **foto** de la alerta (*foto* = **estado actual de la alerta**; no tiene que ver con las evidencias fotográficas de RN-09): `alert_id`, `alert_code`, `measuring_point_id`, `event_type`, `change`, `state`, `current_level`, `max_level`, `blinking`, `management_status`, `classification`, `started_at`, `evaluated_at` y `occurred_at`.
- **`change`** (uno por hora, el más relevante): `OPENED`, `REACTIVATED`, `FINISHED`; si no, `LEVEL_RAISED` o `LEVEL_LOWERED` (incluidas las subidas silenciosas por intervención); si el nivel no cambió, `NO_DATA_STARTED` o `NO_DATA_ENDED`. Una hora sin cambio visible no publica nada.
- **Gestión**: `ManagementService` publica `CLASSIFIED` y `LABELED` solo cuando el paso se guardó (`SAVED`); un conflicto, un reenvío (`ALREADY_RECEIVED`) o una observación libre no publican. Clasificar un rojo publica `blinking = false`: deja de titilar.
- **Cuándo**: igual que el aviso, después del commit, y nunca en el backfill. El puerto `AlertChangePublisher` es opcional (`WithChangePublisher` en `ProcessingService` y `ManagementService`); el adaptador al bus es `busAlertChangePublisher` (`internal/bootstrap`).
- **Orden**: el consumidor aplica nivel y estado solo si `evaluated_at` es más nuevo que el que conoce (en la gestión es la última hora procesada, no cambia); los campos de gestión, solo si `management_status` avanzó.

## API (`/api/alerts`, sin autenticación todavía)

| Método y ruta | Uso |
|---|---|
| `GET /?device=&from=&to=&max_level=&status=&event=&page=` | Historial: 10 por página, inicio descendente, `total`. Las fechas (`dd/mm/aaaa` o `aaaa-mm-dd`) van sobre el **inicio**, de 00:00 a 23:59 hora Colombia; con una sola fecha, el rango queda abierto. `to` < `from` ⇒ 400. La multiselección repite el parámetro o usa comas. `max_level` es sobre el nivel máximo. `device=todos` equivale a no filtrar |
| `GET /export.xlsx?(mismos filtros)` | Excel de la búsqueda (todas las páginas, mismo orden), con hojas "Alertas" y "Parámetros". Más de 10.000 ⇒ 400, sin archivo |
| `GET /pending?limit=5` | Las Sin clasificar más recientes, con su total (panel) |
| `GET /active` | Alertas activas por dispositivo: `level` (la mayor) y `blinking` |
| `GET /{id}` | Detalle en cualquier estado (`{id}` numérico o `ALR-000123`): datos, `levels[]` (colores con `messages[]`), clasificación, etiqueta, observaciones y `available_actions` |
| `GET /devices/{id}/alerts?from=&to=` | Historial del dispositivo: alertas que se solapan con el rango (por defecto los últimos 30 días), cada una con sus colores y mensajes |
| `POST /{id}/classification` | `{classification, observation?, origin, device_identifier?, client_record_id?, recorded_at?}` ⇒ 201 `SAVED` · 409 `SAVED_AS_CONFLICT_OBSERVATION` + `existing` · 200 `ALREADY_RECEIVED` |
| `POST /{id}/label` | `{label, other_text?, observation?, origin, …}` (mismas respuestas) |
| `POST /{id}/observations` | `{text, origin, …}` ⇒ 201 · 200 `ALREADY_RECEIVED` |
| `GET /policies?device=` · `GET /policies/history?device=` · `POST /policies` | Política versionada: `{measuring_point_id?, patch, valid_from?, reason, created_by}`. Sin punto = global. `valid_from` en el pasado ⇒ 400 |

`available_actions`: Sin clasificar ⇒ `CONFIRM`, `FALSE_ALERT`, `VIEW` · Clasificada sin etiqueta ⇒ `ADD_LABEL`, `VIEW` · Clasificada con etiqueta ⇒ `VIEW`. En el historial, `current_level` es `null` si la alerta finalizó ("—").

Errores (`shared/errors.AppError`): 400 `INVALID_INPUT` (y `INVALID_MEASURING_POINT_ID`), 404 `NOT_FOUND`, 409 `CONFLICT`, 500 `CONSULTING_ALERTS` / `PERSISTING_ALERTS` / `CONSULTING_RULE_EVALUATIONS`. Cuerpo máximo: 64 KiB; un campo desconocido ⇒ 400.

> **Sin autenticación (temporal)**, igual que el Rule Engine. No exponer `/api/alerts` fuera de la red de confianza hasta tener auth. Cuando exista: el origen y `created_by` salen del token.

## Política (`alert_policy_versions`)

Sigue el patrón de `rule_threshold_versions`: solo se agregan versiones y nunca hay cambios retroactivos. La creación cierra la vigente en la misma TX (`FOR UPDATE NOWAIT`; concurrencia ⇒ 409) y el `EXCLUDE` impide solapamientos por `COALESCE(measuring_point_id, 0)`. Una versión de un punto reemplaza a la global en ese punto.

**Seed**: la versión global 1 con `valid_from = 2000-01-01`, para que el backfill tenga política en todo el histórico (única excepción retroactiva).

| Parámetro | Inicial | Decisión |
|---|---|---|
| `downgrade_hours` | 6 | D-34 |
| `rojo_requires_classification_to_downgrade` | true | D-35 |
| `merge_window_hours` / `reactivate_only_if_unclassified` | 24 / true | D-31 |
| `intervention_silent_max_level` | NARANJA (CHECK ≠ ROJO) | D-36 |
| `summary_every_hours` / `countdown_hours` | 6 / 3 | D-38 |
| `trend_lookback_hours` / `trend_delta` | 3 / 0,02 | — |
| `oscillation_flips` / `oscillation_window_hours` | 4 / 6 | — |
| `late_notification_max_hours` | 2 | — |

## Eventos

| Tópico | Publicador | Notice propio | Suscripción | Acción |
|---|---|---|---|---|
| `rule_engine.level_evaluated` (v3) | Rule Engine | `entities.LevelNotice` (lo traduce `internal/bootstrap/alert_management.go`) | `alert_management.level_evaluated` | `ProcessingService.Process` (`LIVE`) |

Los eventos son best-effort: `rule_evaluations` es la fuente de verdad. Un duplicado o una hora vieja (`evaluated_at` ≤ `alert_processing_state.last_evaluated_at`) no hace nada.

**Publica** `alert_management.alert_notification` (v1) por cada mensaje `NOTIFICACION` nuevo y `alert_management.alert_changed` (v1) por cada cambio visible, de una hora `LIVE` o `CATCH_UP` y después del commit (ver "Notificaciones"). Los consume el módulo `notifications` (web y push). Flujo completo entre módulos: `backend/FLUJO_RULE_ENGINE_ALERT_MANAGEMENT.MD`.

## Catch-up y backfill (sección 9.2)

- **Catch-up** (`CatchUpScheduler`): corre al arrancar y cada hora en el minuto `ALERT_MANAGEMENT_CATCHUP_MINUTE` (55, después del 50 del Rule Engine). Lee de `rule_evaluations` + `rule_threshold_versions` (modelo propio, solo lectura) las horas posteriores a la última procesada y las procesa con el mismo caso de uso (`CATCH_UP`, publicando avisos). Un punto sin estado se saltea con un Warn ("correr alert_management -backfill"): nunca publica el histórico entero.
- **Backfill**: `cmd/alert_management -backfill [-point] [-from] [-to] [-rebuild]`. Reconstruye sin publicar avisos (`BACKFILL`). Es idempotente: retoma desde la última hora procesada. `-rebuild` borra **todo** lo del punto, **gestión incluida**, y está bloqueado con `AGATHA_ENV=production`.
- En los dos, un salto de `evaluated_at` se procesa como horas `NO_CALCULABLE`, igual que en vivo (como máximo 744 horas inferidas por salto).

**Orden de inicialización**: `baseline -backfill` → `processing -backfill` → `rule_engine -backfill` → `alert_management -backfill` → habilitar `alert_management` en `AGATHA_MODULES` (requiere `rule_engine`).

## Exclusión de la línea base (sección 9.3, invertida)

Alert Management no le entrega nada a Baseline: no hay fachada, ni tabla `known_events`, ni endpoints `/known-events`. **Baseline lee `alerts` con su propio modelo** (`AlertEventsReader`) y excluye **todas** las alertas del punto, en `[started_at − 2 h, ended_at + 2 h)` (una activa, hasta el fin del rango que consulta). Ver `baseline/BASELINE_README.md`.

## Tablas

`alerts`, `alert_level_segments`, `alert_messages`, `alert_observations`, `alert_processing_state`, `alert_policy_versions`. El detalle está en `ARQUITECTURA_LITE.md` §10. No hay FK a tablas de otros módulos. `persistence.Migrate` es idempotente: AutoMigrate + SQL para la columna `code` GENERATED, los índices parciales, los CHECK de gestión, el `EXCLUDE` (con `btree_gist`) y el trigger.

## Decisiones propias (más allá de la especificación)

- `alerts.classified_client_record_id` / `labeled_client_record_id`: dan idempotencia a la clasificación y la etiqueta, no solo a las observaciones.
- `ALMOST_NORMAL` no cita las horas que faltan, porque el evento v3 no trae `exit_hours`. Dice "terminará si baja de {cierre} y se mantiene así".
- `{pk}` sale de `devices.name` (que ya trae "Sensor …"), porque `devices` no tiene abscisa. Sin dispositivo, se usa `MP-<id>`.
- **Los mensajes solo informan el estado y la criticidad; nunca dicen qué hacer.** Se quitaron del texto de la especificación las indicaciones operativas ("Programar recorrido", "Enviar cuadrilla", "avisar al centro de control", "no se requiere salir a campo") y la variable `{zona}`. Además, **el cuerpo no repite el título**: lo complementa. Las dos reglas las verifican tests del catálogo.
- **Los mensajes citan el dato (catálogo v4, `MessageCatalogVersion = 4`).** Todo mensaje del procesamiento horario dice el índice de ruido de la hora en decimal con coma (`messages.FormatValue`: "0,18", con un tercer decimal solo si hace falta, "0,158") y el límite que explica el mensaje. Ese límite puede ser lo normal (`{normal}`, el corte de Amarillo), el corte del nivel (`{corte}`), la salida de Amarillo (`{cierre}`), el índice anterior (`{valor_anterior}`: la última hora calculada o la de `trend_lookback_hours` antes) o el máximo de la alerta (`{valor_max}`). Ejemplo: "Índice de ruido en 0,18 desde las 05:00 del 24/08; lo normal es menos de 0,16." Lo verifica `TestCatalog_ProcessingMessagesCiteTheIndex`, y `TestDecide_MessagesCiteTheData` fija cada texto. "Lleva N h" ya no dice "seguidas", porque con persistencia `WINDOW` N es un conteo en la ventana.
- `alerts.max_indicator_value`: el índice más alto de las horas calculadas de la alerta, con sus reactivaciones. Lo citan el fin, la bajada y el resumen, y lo expone el detalle (`max_indicator_value`). `Migrate` completa las filas previas con el mayor índice que conocen: la apertura, el último y el de sus mensajes. Es un valor aproximado; `-rebuild` lo deja exacto.
- La duración ("Duró N h", "lleva N h" y `duration_hours` en la API) es tiempo transcurrido sin las pausas entre fin y reactivación (`Alert.DurationHours`), no `hours_calculated`.
- Después de una subida no se emiten las reglas 5 a 9 en esa hora. Abrir o reactivar no emite otras reglas.
- Sin estado previo y en mitad de un episodio (módulo habilitado tarde), la primera hora abre la alerta igual.
- El Excel se implementó solo con la stdlib (no hay dependencias nuevas en `go.mod`).
- `late` viaja en el evento en vez de filtrar acá: la política es de Alert Management, pero qué canal se omite lo decide Notification.

- La sección 9.3 (fachada `KnownEvents` + tabla `known_events` + `/known-events`) **no se implementa así**: los eventos manuales del analista son un insumo de Baseline, no gestión de alertas. Baseline ya no tiene lista estática: lee `alerts` directo (acople de esquema, solo lectura).

## Configuración

`AGATHA_ENV` (`production` bloquea `-rebuild`), `ALERT_MANAGEMENT_CATCHUP_MINUTE` (55), `ALERT_MANAGEMENT_BACKFILL_BATCH_SIZE` (500).

## Reglas de Import (Aislamiento)

No importa `rule_engine` (ni su `contract/`), `processing`, `ingestion`, `baseline` ni `shared/eventbus` (`depguard` `alert-management-isolation`). Lee `rule_evaluations`, `rule_threshold_versions` y `devices` con modelos propios (acople de esquema). La suscripción, la traducción del evento, y el adaptador `NotificationPublisher` → bus viven en `internal/bootstrap`. **No conoce a Baseline**: Baseline lee `alerts` por su cuenta. El consumidor (Notification) tampoco importa este módulo ni su `contract/`: define su propio tipo y `internal/bootstrap` traduce y suscribe (guardrail 6).

## Tests

- `domain/services/decide_test.go`: escenarios 1 a 15 de la sección 10 sobre `Decide`. Cubre apertura con fecha inicio, cuenta regresiva, subida por duración y de golpe, bajada a las 6 h con 3 colores, rojo retenido y rojo clasificado, banda de cierre, oscilación, hueco NO_CALC, Falsa alerta que sigue notificando, intervención silenciosa, reactivación (con y sin superar el máximo), alerta nueva tras clasificada o tras 24 h, marca preliminar, resumen y tendencia.
- `domain/messages/catalog_test.go`: variables declaradas, lenguaje de campo y error de render ante una variable faltante.
- `application/services/*_test.go`: el procesamiento con el store en memoria (vivo publica el aviso, duplicados, hueco por salto, backfill sin publicar, catch-up atrasado marcado `late`, fallo de BD sin aviso, fallo al publicar sin perder la alerta, gestión no pisada, reactivación); catch-up y backfill (idempotencia y rebuild); gestión (escenarios 16 a 18); filtros, exportación (19 y 19b), activas y política.
- `infrastructure/http/alert_handler_test.go`: los flujos HTTP de clasificación y etiqueta, campos desconocidos, que las rutas literales ganen a `{id}`, que no existan `/notifications` ni `/push-devices`, la exportación y la política.
- `infrastructure/{export,scheduling,persistence/mappers}`: xlsx bien formado, scheduler, y mappers de ida y vuelta (incluido el `LevelNotice` desde la fila en modo WINDOW).
- `internal/bootstrap/alert_management_test.go`: traducción del contrato, y **de punta a punta con el bus real** (Processing → Rule Engine → Alert Management → `alert_management.alert_notification`: abre, sube, hueco, cierra; solo OPEN y UP_NARANJA_FAST salen como avisos).
- **Sin pruebas de integración contra PostgreSQL todavía**: la migración, el trigger, los CHECK, el `EXCLUDE` y las consultas GORM no se corrieron contra una BD.
