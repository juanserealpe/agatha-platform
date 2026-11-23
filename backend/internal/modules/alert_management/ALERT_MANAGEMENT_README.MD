# Modulo: Alert Management

**Responsabilidad**: convertir el nivel horario que publica el Rule Engine (`rule_engine.level_evaluated` v3) en **alertas** para el operador (web) y el usuario de campo (app móvil). Desde el catálogo v7 procesa también el movimiento del sensor (`rule_engine.motion_evaluated` v1 y v2; desde el catálogo v8, motion-v2) como **alertas de MOVIMIENTO separadas**: FUGA y MOVIMIENTO nunca se combinan; si se solapan son dos alertas (ver "Movimiento"). Mantiene su nivel actual y su nivel máximo. Registra la línea de tiempo (colores y mensajes), decide qué mensajes son **notificaciones** (alertas nuevas y subidas de nivel) y guarda la gestión del operador: clasificación, etiqueta y observaciones.

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
| 1 (siempre) | `OPEN` (variante por nivel, o `SIN_FIRMA` si la hora quedó limitada por la planitud), `UP_NARANJA_PERSIST` / `UP_NARANJA_FAST` / `UP_ROJO` (variante "sostenido N h"; con la planitud recién confirmada, variante `*_CONFIRMADA`), `REACTIVATED`, `NEW_AFTER_CLASSIFIED`, `UP_DURING_INTERVENTION` | Crear, subir, reactivar. "Por duración" = motivo `LEAK_PERSISTENCE` en la hora |
| 2 | `NO_CALC_START` / `NO_CALC_END` | Empieza o termina una racha sin cálculo |
| 3 | `DOWNGRADE` / `ROJO_HELD` | Baja tras 6 h / el rojo sin clasificar no baja (una vez) |
| 4 | `OSCILLATING` | `oscillation_flips` (4) cambios entre "sobre el umbral" y "banda de cierre" en `oscillation_window_hours` (6). Silencia 5 y 6 hasta 6 h sin cambios |
| 4b | `NO_LEAK_SHAPE` | La hora quedó limitada a Amarillo por la planitud y la alerta está en Amarillo: **una vez por racha** de horas limitadas (ver "Confirmación por planitud"). Actualización de la línea de tiempo, no un aviso |
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

Decisión (2026-10-26, cambia la propuesta técnica del documento, que excluía solo Confirmadas y mantenimiento): **toda alerta se excluye de la línea base**, cualquiera sea su clasificación, incluidas Sin clasificar y activas. La regla vive en Baseline (`baseline_exclusion_periods`, alimentada por `alert_changed`).

## Notificaciones (sección 5): eventos para el módulo de notificaciones

Alert Management decide **qué** es una notificación (`kind = NOTIFICACION` en el catálogo y las reglas de `Decide`: apertura, subidas, alerta nueva tras clasificada; las subidas silenciosas por intervención no lo son). **Cómo** se entrega no es asunto de este módulo.

- **Evento**: `alert_management.alert_notification` (`contract.AlertNotification`, `schema_version` 1), uno por cada mensaje `NOTIFICACION` **nuevo**. Lleva `message_id` (clave de idempotencia), `alert_id`, `alert_code`, `measuring_point_id`, `event_type`, `started_at`, `evaluated_at`, `code`, `level`, `max_level`, `alert_state`, `classification` (vacío = sin clasificar; el consumidor omite el push de `FALSA_ALERTA`), `title` y `body` ya renderizados, `indicator_value`, `source` (`LIVE` | `CATCH_UP`), `late`, `policy_version_id` y `occurred_at`.
- **Cuándo**: después del commit de la hora. Si la TX falla no se publica nada; si falla la publicación, la alerta ya quedó guardada (se loguea). En el backfill y en reprocesos (el mensaje ya existía) no se publica.
- **`late`**: la hora llegó con más de `late_notification_max_hours` (2) de atraso (catch-up, Processing reponiéndose de una caída). La especificación pide no enviar push en ese caso; lo aplica el consumidor.
- **Best-effort**: el bus es en memoria (un evento se pierde si el proceso cae entre el commit y la publicación). La fuente de verdad es `alert_messages` (`kind = 'NOTIFICACION'`, índice `ix_alert_messages_kind`): el módulo de notificaciones puede reconciliar por `id`.
- El adaptador del puerto `NotificationPublisher` al bus es `busAlertNotificationPublisher` (`internal/bootstrap`); sin suscriptores, publicar es un no-op.

### Cambios visibles: `alert_management.alert_changed`

Para que el mapa y el panel se actualicen en tiempo real sin consultar la API.

- **Evento**: `contract.AlertChanged` (`schema_version` 1), una **foto** de la alerta (*foto* = **estado actual de la alerta**; no tiene que ver con las evidencias fotográficas de RN-09): `alert_id`, `alert_code`, `measuring_point_id`, `event_type`, `change`, `state`, `current_level`, `max_level`, `blinking`, `management_status`, `classification`, `leak_signature` (objeto o `null`, ver "Confirmación por planitud"), `started_at`, `ended_at` (UTC, `null` mientras está en curso; una reactivación lo vuelve a `null`; campo aditivo, sin cambio de `schema_version`), `evaluated_at` y `occurred_at`. Consumidores: Notifications (web) y Baseline (exclusiones).
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

## Movimiento (`REQ_ALERTAS_MOVIMIENTO` v1.0, §7, §8 y §11.2)

Decisión (2026-11-19, cambia §1, §7 y §9 del requerimiento): **FUGA y MOVIMIENTO no se combinan**. Cada flujo tiene sus propias alertas: si se solapan son dos alertas, una de ruido y otra de movimiento, cada una con sus colores, mensajes, gestión y reactivación. Una alerta de MOVIMIENTO usa el mismo modelo de datos que una de FUGA (`alerts`, `alert_level_segments`, `alert_messages`, `alert_processing_state`), con `event_type = 'MOVIMIENTO'`. Lo único compartido es la **temperatura**: los `TEMP_*` de una hora de movimiento salen en la alerta de MOVIMIENTO y, si hay una alerta de FUGA vigente, también en ella, con el mismo tipo de mensaje (Notificación o Actualización).

- **Datos (§7.1)**: contexto del movimiento en `alerts` (`motion_state`, `temp_state`, `temp_stable_from`, `temp_stable_to`, `last_motion_evaluated_at`). En una alerta de MOVIMIENTO se copia de su última hora calculada; en una de FUGA solo se llenan `temp_state` y el tramo `temp_stable_*` cuando le llega un `TEMP_*`. `ux_alerts_one_active` es `UNIQUE (measuring_point_id, event_type) WHERE state = 'ACTIVA'`: una activa por (punto, evento). `alert_processing_state` tiene una fila (punto, FUGA) y otra (punto, MOVIMIENTO). No hay `alerts.events` ni `alert_processing_state.in_episode`: la migración los borra si quedaron de la v7 acoplada, y reemplaza el índice por punto si existía.
- **Concurrencia (§7.2)**: el mismo lock por punto serializa los dos flujos; cada uno tiene su `last_evaluated_at`. Backfill y catch-up intercalan por hora: primero FUGA, después MOVIMIENTO (así un `TEMP_*` llega a la FUGA que está vigente en esa hora).
- **FUGA (§7.3)**: `Decide` es el de antes de la v7, sin acople: no lee nada del movimiento, abre con `OPEN`, finaliza con su CLEARED y se reactiva solo sobre la última FINALIZADA **de FUGA** (`AlertTx.GetLastFinished(punto, evento)`). Se eliminó `UP_DURING_INTERVENTION`: con `MANTENIMIENTO_PROGRAMADO` las subidas emiten su `UP_*` (CA-5; `intervention_silent_max_level` queda en la política sin uso).
- **MOVIMIENTO (§7.4, §7.5)**: `DecideMotion` sobre la alerta de MOVIMIENTO del punto, en este orden: `MOV_OPEN` (alerta nueva o reactivación de la última FINALIZADA de MOVIMIENTO, siempre Amarillo), `MOV_DISPLACED`, `TEMP_STABILIZED`, `MOV_RETURN`, `TEMP_VARIES_AGAIN`, `TEMP_NOT_STABILIZED`, `TEMP_STILL_STABLE` y, con CLEARED, `END_NORMAL/MOVIMIENTO` (finaliza). `started_at`: con `MOV_ABRUPT` t − (hours − 1) h, con `MOV_SHIFT_GAP` el fin de la hora de `gap_from`, si no t. La política (ventana de reactivación, atraso) es la de FUGA. El inicio del desplazamiento en curso va en `alerts.runtime`: en la hora del retorno el evento ya no trae `shift.started_at`.
- **Temperatura en la alerta de FUGA**: `DecideTemperature` (puro) replica en la FUGA ACTIVA los `TEMP_*` de la hora (sin `MOV_*`), guarda `temp_state` y el tramo, y no cambia su color, su estado ni su fin. Una FUGA que no está vigente en esa hora no recibe nada.
- **Modo sombra (CA-7)**: el Rule Engine no publica con `alerts_enabled = false`, `MotionSource` solo lee horas de versiones con `alerts_enabled` y `ProcessMotion` ignora las que lleguen así.
- **Huecos en vivo**: si un evento de movimiento llega con horas faltantes desde la última procesada, `ProcessMotion` las lee de `motion_evaluations` antes (las de movimiento tienen fila siempre, también NO_CALCULABLE).
- **Gestión (§7.6)**: `CLASSIFIED` / `LABELED` (kind GESTION) con `evaluated_at` = la hora guardada al segundo (= `classified_at` / `labeled_at`), en el color vigente; `params` agrega `classification` o `label`/`other_text`, `origin`, `device_identifier`, `recorded_at` y `observation`. Un 409, un reenvío o una observación suelta no crean mensaje. `GET /{id}` ya ordenaba por `evaluated_at`.
- **Avisos**: `alert_notification` y `alert_changed` siguen en v1; `event_type` del aviso es el de la **alerta** (un `TEMP_*` replicado en la FUGA sale como FUGA, con el ID de esa alerta).
- **Catálogo v7 (§8)** (los textos de movimiento los reemplaza el v8, abajo): textos de §8.1 y §8.2 (`IsMotionKey` marca los de movimiento), sin `LEAK_JOINED`, `MOV_JOINED` ni `PART_NORMAL`, que solo existían por el acople. Excepción: `TEMP_STABILIZED` decía "Pasó lo mismo en cierres anteriores", que §8.3 prohíbe ("cierre"); quedó "Pasó lo mismo antes en este punto, pero no está confirmado".
- **Rebuild (§11.2)**: `-rebuild` borra **todo** el punto (los dos flujos y sus estados). Si el punto tiene una **alerta vigente** (`ended_at IS NULL`), aborta **sin escribir** con `ErrActiveAlertPresent`, y no hay flag que lo saltee: el rebuild la recrearía con otro código y su período en `baseline_exclusion_periods` quedaría abierto para siempre; se espera a que finalice. Si alguna alerta tiene clasificación, etiqueta u observaciones, aborta **sin escribir** salvo `-drop-management`. Cuenta antes (`AlertReset.CountPoint`) alertas, vigentes, mensajes, clasificaciones, etiquetas y observaciones, lo registra y el comando lo imprime.

### motion-v2: `DecideMotion` v8 y catálogo v8 (`REQ_MOVIMIENTO_UMBRALES_V2` §9 y §10)

El Rule Engine separa el nivel **calculado** (`table_level`, por la tabla de lo normal del dispositivo) del **enviado** (`computed_level`: Naranja y Rojo en `OBSERVATION` salen como el nivel activo inferior). Referencia: `rule_engine/domain/services/testdata/motion_v2/messages_v2_ref.py`; si el texto y la referencia difieren, manda la referencia.

- **Datos (§9.1)**: `alerts.motion_history` (jsonb, el `history` del OPENED que abrió o reactivó la alerta) y `alerts.motion_table_level` (nivel calculado máximo, CHECK AMARILLO | NARANJA | ROJO), aditivas, NULL en FUGA; se exponen en `GET /api/alerts/{id}` (`motion_history`, `motion_table_level`). `MotionNotice` agrega `TableLevel`, `EpisodeKind`, `RulesHit`, `History` y `SchemaVersion`; `MotionReason.Data` guarda el `data` tal como llegó (`Num`, `Int`, `Str`, `Bool`, `Time`, que caen a los campos tipados de v1 si falta la clave). `MotionSource` lee además `table_level`, `episode_kind`, `history` y `rules_hit` de `motion_evaluations` (el Rule Engine debe migrar antes).
- **Reglas (§9.2)**: OPENED abre (o reactiva) en el nivel **enviado** y guarda historial y nivel calculado; `MOV_LEVEL_UP` con `sent_to` > `sent_from` abre un color nuevo (`MOV_LEVEL_UP`, Notificación); si no, solo sube `motion_table_level` (`MOV_LEVEL_OBSERVED`, Actualización); `MOV_CUMULATIVE` fuera del OPENED ⇒ `MOV_SLOW_ONGOING`; `MOV_DAILY` ⇒ resumen (kind RESUMEN), **también en una hora NO_CALCULABLE**; CLEARED ⇒ `END_NORMAL/MOVIMIENTO` con el cambio total, el nivel máximo enviado y, si el calculado lo superó, "(por su magnitud correspondía a …)". Una alerta de MOVIMIENTO nunca baja ni emite reglas de FUGA. En la hora OPENED solo sale `MOV_OPEN` (como la referencia). Orden de la hora: `MOV_DISPLACED`, `MOV_LEVEL_UP`/`OBSERVED`, `TEMP_STABILIZED`, `MOV_RETURN`, `TEMP_VARIES_AGAIN`, `TEMP_NOT_STABILIZED`, `TEMP_STILL_STABLE`, `MOV_SLOW_ONGOING`, `MOV_DAILY`, `END_NORMAL`. `started_at`: RAPIDO t − (hours − 1) h, HUECO el fin de la hora de `gap_from`, SALTO y ACUMULADO t. Duración del fin: con reactivación, "Duró X desde que se reactivó; la alerta en total, Y". `params` de cada mensaje de movimiento guarda `datos` (el `data` del motivo, para regenerar el texto).
- **Catálogo v8 (§10)** (`MessageCatalogVersion = 8`, también en los mensajes de FUGA, cuyos textos no cambian): `MOV_OPEN` con las variantes `SALTO`, `HUECO`, `RAPIDO` y `ACUMULADO`; `MOV_LEVEL_UP`, `MOV_LEVEL_OBSERVED`, `MOV_SLOW_ONGOING`, `MOV_DAILY` (y `/SIN_DATOS`, `/PARCIAL`), `MOV_DISPLACED` (`SALTO`, `HUECO`, `RAPIDO`), `MOV_RETURN` y `END_NORMAL/MOVIMIENTO` nuevos; los `TEMP_*` no cambian. El número va en la frase (sin "Dato:"). Las piezas compuestas (`{entre}`, `{calculado}`, `{historial}` con su salto de línea, `{acumulado}`) y los formatos (grados medio alejándose de cero sobre 3 decimales, velocidades con 3, `FormatSpan` medio hacia arriba, `FormatWindow`) están en `domain/messages/motion_v8.go`. Los nombres de variable solo admiten letras y `_` (la regex de `Render`): `{dp_dia}`, no `{dp_24h}`.
- **Evento v1**: el tipo se deduce de los motivos, el nivel calculado es el enviado, sin historial y, en RAPIDO, lo normal es la regla fija de v1 (0,04° en 3 h). No hay un segundo juego de textos v7: tras el despliegue el Rule Engine solo emite v2 y el rebuild reemplaza las filas motion-v1.

## Eventos

| Tópico | Publicador | Notice propio | Suscripción | Acción |
|---|---|---|---|---|
| `rule_engine.level_evaluated` (v3) | Rule Engine | `entities.LevelNotice` (lo traduce `internal/bootstrap/alert_management.go`) | `alert_management.level_evaluated` | `ProcessingService.Process` (`LIVE`) |
| `rule_engine.motion_evaluated` (v1) | Rule Engine | `entities.MotionNotice` (`toMotionNotice`; los motivos con `entities.ParseMotionReason`) | `alert_management.motion_evaluated` | `ProcessingService.ProcessMotion` (`LIVE`) |

Los eventos son best-effort: `rule_evaluations` es la fuente de verdad. Un duplicado o una hora vieja (`evaluated_at` ≤ `alert_processing_state.last_evaluated_at`) no hace nada.

**Publica** `alert_management.alert_notification` (v1) por cada mensaje `NOTIFICACION` nuevo y `alert_management.alert_changed` (v1) por cada cambio visible, de una hora `LIVE` o `CATCH_UP` y después del commit (ver "Notificaciones"). Los consume el módulo `notifications` (web y push). Flujo completo entre módulos: `backend/FLUJO_RULE_ENGINE_ALERT_MANAGEMENT.MD`.

## Catch-up y backfill (sección 9.2)

- **Catch-up** (`CatchUpScheduler`): corre al arrancar y cada hora en el minuto `ALERT_MANAGEMENT_CATCHUP_MINUTE` (55, después del 50 del Rule Engine). Lee de `rule_evaluations` + `rule_threshold_versions` (modelo propio, solo lectura) las horas posteriores a la última procesada y las procesa con el mismo caso de uso (`CATCH_UP`, publicando avisos). Un punto sin estado se saltea con un Warn ("correr alert_management -backfill"): nunca publica el histórico entero.
- **Backfill**: `cmd/alert_management -backfill [-point] [-from] [-to] [-rebuild [-drop-management]]`. Reconstruye sin publicar avisos (`BACKFILL`), con FUGA (`rule_evaluations`) y MOVIMIENTO (`motion_evaluations`, `MotionSource`) intercalados por hora. Es idempotente: retoma desde la última hora procesada de cada flujo. `-rebuild` borra **todo** lo del punto y está bloqueado con `AGATHA_ENV=production` y con una alerta vigente en el punto; con gestión en el punto exige `-drop-management` (§11.2).
- El catch-up también lee `motion_evaluations` desde el `last_evaluated_at` de la fila (punto, MOVIMIENTO); un flujo sin estado se saltea.
- En los dos, un salto de `evaluated_at` se procesa como horas `NO_CALCULABLE`, igual que en vivo (como máximo 744 horas inferidas por salto).

**Orden de inicialización**: `baseline -backfill` → `processing -backfill` → `rule_engine -backfill` → `alert_management -backfill` → habilitar `alert_management` en `AGATHA_MODULES` (requiere `rule_engine`).

## Exclusión de la línea base (sección 9.3, invertida)

Alert Management no conoce a Baseline: no hay fachada ni endpoints `/known-events`. **Baseline guarda su propia copia** en `baseline_exclusion_periods`: bootstrap le pasa cada `alert_changed` (upsert por `alert_code` con `started_at`/`ended_at`) y `cmd/baseline -seed-exclusions` copia `alerts` una vez. Excluye **todas** las alertas del punto en `[started_at − 2 h, ended_at + 2 h)`. Como esa copia no se borra, un `-rebuild` de alertas no cambia las exclusiones de la línea base (el backfill tampoco publica). Ver `baseline/BASELINE_README.md`.

## Tablas

`alerts`, `alert_level_segments`, `alert_messages`, `alert_observations`, `alert_processing_state`, `alert_policy_versions`. El detalle está en `ARQUITECTURA_LITE.md` §10. No hay FK a tablas de otros módulos. `persistence.Migrate` es idempotente: AutoMigrate + SQL para la columna `code` GENERATED, los índices parciales, los CHECK de gestión, el `EXCLUDE` (con `btree_gist`) y el trigger.

## Decisiones propias (más allá de la especificación)

- `alerts.classified_client_record_id` / `labeled_client_record_id`: dan idempotencia a la clasificación y la etiqueta, no solo a las observaciones.
- `ALMOST_NORMAL` no cita las horas que faltan, porque el evento v3 no trae `exit_hours`. Dice "terminará si baja de {cierre} y se mantiene así".
- `{pk}` sale de `devices.name` (que ya trae "Sensor …"), porque `devices` no tiene abscisa. Sin dispositivo, se usa `MP-<id>`.
- **Los mensajes solo informan el estado y la criticidad; nunca dicen qué hacer.** Se quitaron del texto de la especificación las indicaciones operativas ("Programar recorrido", "Enviar cuadrilla", "avisar al centro de control", "no se requiere salir a campo") y la variable `{zona}`. Además, **el cuerpo no repite el título**: lo complementa. Las dos reglas las verifican tests del catálogo.
- **Los avisos empiezan con una frase simple (catálogo v5, `MessageCatalogVersion = 5`).** Los cuerpos de los avisos que llegan al banner web y al push (`OPEN`, `NEW_AFTER_CLASSIFIED`, `UP_NARANJA_PERSIST`, `UP_NARANJA_FAST`, `UP_ROJO`, `REACTIVATED`) dicen primero qué pasa en palabras simples y al final el dato: "El índice de ruido es demasiado alto desde las 05:00 del 24/08. Dato: 0,41 (Rojo desde 0,35; lo normal es menos de 0,16)." Los demás mensajes de la línea de tiempo no cambian. Las variables y los títulos son los mismos que en v4. Los mensajes ya guardados conservan su texto y su versión.
- **Los mensajes citan el dato (desde el catálogo v4).** Todo mensaje del procesamiento horario dice el índice de ruido de la hora en decimal con coma (`messages.FormatValue`: "0,18", con un tercer decimal solo si hace falta, "0,158") y el límite que explica el mensaje. Ese límite puede ser lo normal (`{normal}`, el corte de Amarillo), el corte del nivel (`{corte}`), la salida de Amarillo (`{cierre}`), el índice anterior (`{valor_anterior}`: la última hora calculada o la de `trend_lookback_hours` antes) o el máximo de la alerta (`{valor_max}`). Ejemplo (v5): "El índice de ruido está por encima de lo normal desde las 05:00 del 24/08. Dato: 0,18 (lo normal es menos de 0,16)." Lo verifica `TestCatalog_ProcessingMessagesCiteTheIndex`, y `TestDecide_MessagesCiteTheData` fija cada texto. "Lleva N h" ya no dice "seguidas", porque con persistencia `WINDOW` N es un conteo en la ventana.
- **Confirmación por planitud (catálogo v6, `MessageCatalogVersion = 6`).** El Rule Engine puede limitar una hora a Amarillo (motivo `NARANJA_NOT_CONFIRMED`, con `from` Naranja o Rojo) cuando `d_pl < d_pl_min`. Alert Management **no cambia sus reglas** (abrir, cerrar, bajar tras 6 h, Rojo sin clasificar, reactivar, gestión): la planitud solo limita el color. Lo que agrega es la explicación, con la "firma de fuga" (el % de registros de las últimas 24 h con planitud alta, `d_pl`, frente al mínimo `d_pl_min`; porcentajes con `messages.FormatPercent`: 4,6 → "4,6 %", 10 → "10 %"). Los textos nunca dicen qué hacer ni dan veredicto (`TestCatalog_NeverGivesALeakVerdict`):
  - `OPEN` variante `SIN_FIRMA` (aviso): la alerta abre en Amarillo porque la hora estaba limitada. Título "Alerta nueva en {pk}: nivel Amarillo". Cuerpo: "Por el índice de ruido la alerta estaría en {from} desde las {hora_inicio}, pero solo el {d_pl} de los registros de las últimas 24 h tiene firma de fuga (se necesita {d_pl_min}). Dato: {valor} ({from} desde {corte}; lo normal es menos de {normal})." `{from}` es el nivel que el índice daría solo y `{corte}`, su corte.
  - `NO_LEAK_SHAPE` (actualización, **no** se publica como aviso). Título "Se mantiene en Amarillo". Cuerpo: "Por el índice de ruido la alerta estaría en {from}, pero solo el {d_pl} de los registros de las últimas 24 h tiene firma de fuga (se necesita {d_pl_min}). Dato: {valor} ({from} desde {corte})." Ejemplo (14/11 12:00, Rojo, índice 0,36, `d_pl` 4,6, mínimo 10): "Por el índice de ruido la alerta estaría en Rojo, pero solo el 4,6 % de los registros de las últimas 24 h tiene firma de fuga (se necesita 10 %). Dato: 0,36 (Rojo desde 0,35)."
  - `UP_NARANJA_PERSIST`, `UP_NARANJA_FAST`, `UP_ROJO` y `UP_ROJO` sostenido: cuando el nivel sube y la **hora anterior de la alerta estaba limitada** (la planitud acaba de confirmarse), se usa la variante `CONFIRMADA` (`PERSIST_CONFIRMADA` para el Rojo sostenido; el código guardado sigue siendo `UP_*` y el título no cambia). El cuerpo se antepone con: "El {d_pl} de los registros de las últimas 24 h tiene firma de fuga (mínimo {d_pl_min}). " Los `params` guardados incluyen `d_pl` y `d_pl_min`.
  - **Deduplicación de `NO_LEAK_SHAPE`**: la racha son las horas calculadas seguidas limitadas (`alerts.runtime.limited` guarda si la hora anterior lo estaba y `runtime.no_leak_sent`, si el mensaje ya salió en la racha). Sale una vez, en la primera hora de la racha en que la alerta está en Amarillo; una hora **no limitada** reinicia la racha, y una hora sin cálculo no la corta. Si la alerta **abrió** limitada (`OPEN/SIN_FIRMA`), esa apertura ya cuenta como el mensaje de la racha. Tiene la prioridad 4b: si una regla anterior (sin cálculo, bajada, oscilación) ocupa la hora, se intenta en la siguiente; gana a las reglas 5 a 9 de esa hora.
  - Si la hora trae `d_pl` o `d_pl_min` nulos, no hay con qué citar la firma y se usan los textos de siempre.
  - El `Notice` (`entities.LevelNotice`) lleva `DPl`, `DPlMin` y `LimitedFrom` (el `from` del motivo `NARANJA_NOT_CONFIRMED`). Los traducen `bootstrap.toLevelNotice` (evento, contrato v4 del Rule Engine) y `mappers.ToLevelNotice` (catch-up y backfill: `rule_evaluations.d_pl`, `rule_threshold_versions.d_pl_min` y el motivo guardado), con las mismas reglas, así que `-backfill -rebuild` regenera los mensajes v6.
  - **`-rebuild` borra la gestión del operador** (clasificaciones, etiquetas y observaciones del punto): hay que **exportarla antes** (`GET /api/alerts/export.xlsx`) si interesa conservarla. Los mensajes ya guardados conservan su texto y su versión; solo un `-rebuild` los regenera.
- **`leak_signature`** (solo backend): objeto `{ "d_pl": número|null, "d_pl_min": número, "confirmed": bool }` de la **última hora calculada** de la alerta; `null` si la versión de umbrales no define `d_pl_min` (la regla de planitud no está activa para el punto), si no hay alerta o si aún no hay datos. `confirmed` = la hora no se limitó y `d_pl >= d_pl_min`. Una hora sin cálculo no la borra. Se guarda en `alerts` con tres columnas aditivas (`leak_d_pl`, `leak_d_pl_min`, `leak_confirmed`; las alertas anteriores quedan en NULL hasta el siguiente procesamiento o un `-rebuild`), las crea `Migrate` con AutoMigrate. Se expone en la fila y el detalle de `/api/alerts` y en el evento `alert_management.alert_changed`.
- `alerts.max_indicator_value`: el índice más alto de las horas calculadas de la alerta, con sus reactivaciones. Lo citan el fin, la bajada y el resumen, y lo expone el detalle (`max_indicator_value`). `Migrate` completa las filas previas con el mayor índice que conocen: la apertura, el último y el de sus mensajes. Es un valor aproximado; `-rebuild` lo deja exacto.
- La duración ("Duró N h", "lleva N h" y `duration_hours` en la API) es tiempo transcurrido sin las pausas entre fin y reactivación (`Alert.DurationHours`), no `hours_calculated`.
- Después de una subida no se emiten las reglas 5 a 9 en esa hora. Abrir o reactivar no emite otras reglas.
- Sin estado previo y en mitad de un episodio (módulo habilitado tarde), la primera hora abre la alerta igual.
- El Excel se implementó solo con la stdlib (no hay dependencias nuevas en `go.mod`).
- `late` viaja en el evento en vez de filtrar acá: la política es de Alert Management, pero qué canal se omite lo decide Notification.

- La sección 9.3 (fachada `KnownEvents` + tabla `known_events` + `/known-events`) **no se implementa así**: los eventos manuales del analista son un insumo de Baseline, no gestión de alertas. Baseline ya no tiene lista estática: lee `alerts` directo (acople de esquema, solo lectura).

## Configuración

`AGATHA_ENV` (`production` bloquea `-rebuild`; `-drop-management` solo con `-rebuild`), `ALERT_MANAGEMENT_CATCHUP_MINUTE` (55), `ALERT_MANAGEMENT_BACKFILL_BATCH_SIZE` (500).

## Reglas de Import (Aislamiento)

No importa `rule_engine` (ni su `contract/`), `processing`, `ingestion`, `baseline` ni `shared/eventbus` (`depguard` `alert-management-isolation`). Lee `rule_evaluations`, `rule_threshold_versions` y `devices` con modelos propios (acople de esquema). La suscripción, la traducción del evento, y el adaptador `NotificationPublisher` → bus viven en `internal/bootstrap`. **No conoce a Baseline**: Baseline lee `alerts` por su cuenta. El consumidor (Notification) tampoco importa este módulo ni su `contract/`: define su propio tipo y `internal/bootstrap` traduce y suscribe (guardrail 6).

## Tests

- `domain/services/decide_test.go`: escenarios 1 a 15 de la sección 10 sobre `Decide`. Cubre apertura con fecha inicio, cuenta regresiva, subida por duración y de golpe, bajada a las 6 h con 3 colores, rojo retenido y rojo clasificado, banda de cierre, oscilación, hueco NO_CALC, Falsa alerta que sigue notificando, intervención silenciosa, reactivación (con y sin superar el máximo), alerta nueva tras clasificada o tras 24 h, marca preliminar, resumen y tendencia.
- `domain/messages/catalog_test.go`: variables declaradas, lenguaje de campo y error de render ante una variable faltante.
- `application/services/*_test.go`: el procesamiento con el store en memoria (vivo publica el aviso, duplicados, hueco por salto, backfill sin publicar, catch-up atrasado marcado `late`, fallo de BD sin aviso, fallo al publicar sin perder la alerta, gestión no pisada, reactivación); catch-up y backfill (idempotencia y rebuild); gestión (escenarios 16 a 18); filtros, exportación (19 y 19b), activas y política.
- `infrastructure/http/alert_handler_test.go`: los flujos HTTP de clasificación y etiqueta, campos desconocidos, que las rutas literales ganen a `{id}`, que no existan `/notifications` ni `/push-devices`, la exportación y la política.
- `infrastructure/{export,scheduling,persistence/mappers}`: xlsx bien formado, scheduler, y mappers de ida y vuelta (incluido el `LevelNotice` desde la fila en modo WINDOW).
- `internal/bootstrap/alert_management_test.go`: traducción del contrato, y **de punta a punta con el bus real** (Processing → Rule Engine → Alert Management → `alert_management.alert_notification`: abre, sube, hueco, cierra; solo OPEN y UP_NARANJA_FAST salen como avisos).
- **Movimiento**:
  - `domain/services/decide_motion_test.go`: los 17 escenarios de §9.2 (CA-12) con FUGA y MOVIMIENTO como alertas separadas (la FUGA da lo mismo que sin movimiento, más los `TEMP_*` de su tramo vigente), más `SOLO_MOVIMIENTO`, reactivación por evento y `DecideTemperature`; `decide_test.go` agrega CA-5.
  - `domain/messages/catalog_test.go`: dato en todo `TEMP_*`, palabras prohibidas, 19 claves de movimiento, versión 8, sin `UP_DURING_INTERVENTION`, formatos, texto de gestión. `motion_v8_test.go` (CA-12, §10.5): formatos de §10.1 (incluido 0,285 → +0,29°, que la dorada no cubre), palabras prohibidas y sin "Dato:" en movimiento, nombres de variable que `Render` reemplaza, `{calculado}` vacío con el nivel activo.
  - `domain/services/decide_motion_v8_golden_test.go`: **prueba dorada AM (CA-3)**: las 5 salidas del Rule Engine (`motion_v2_expected*.csv`) por `DecideMotion` v8 dan las alertas (inicio, fin, reactivaciones, nivel enviado y calculado) y los textos exactos de `alert_messages_v2_expected*.json` (`TEMP_*` solo por código y hora); verificada por mutación. Además: todo texto de movimiento renderizado cita grados (salvo el resumen sin datos o parcial sin total, como la referencia), sin variables sin reemplazar y con `params.datos`.
  - `internal/bootstrap/motion_v2_alerts_test.go`: de punta a punta RE → evento v2 → AM. CA-7: con `send_naranja = ACTIVE`, `rapido_grande` abre en Amarillo, `MOV_LEVEL_UP` a Naranja (Notificación) a las 17:00 del 10/09 con color nuevo y termina en Naranja; con observación, `MOV_LEVEL_OBSERVED`, un solo color, `motion_table_level` Naranja y `motion_history` guardado.
  - `infrastructure/persistence/mappers/motion_mappers_test.go`: columnas v2 de `motion_evaluations` y `motion_history` / `motion_table_level` ida y vuelta.
  - `application/services/motion_processing_test.go`: modo sombra (CA-7), duplicado (CA-8), horas perdidas en vivo, intercalado FUGA primero, guarda del rebuild y borrado de la gestión (CA-9, CA-10), mensajes de gestión a su hora (CA-3) una ACTIVA por (punto, evento) (CA-2) y la réplica de la temperatura en la FUGA vigente con su aviso.
  - `internal/bootstrap/motion_alerts_test.go`: **las 9 alertas de §12.3** con el movimiento real (`MotionAssess` sobre `motion_input.csv`, traducido como el evento en vivo) y FUGA sintética con los inicios, fines y niveles de la tabla; con `rule_engine/domain/services/testdata/motion/rule_evaluations_11556.csv` corre además con el histórico real de FUGA (si no está, se saltea).
- **Sin pruebas de integración contra PostgreSQL todavía**: la migración, el trigger, los CHECK, el `EXCLUDE` y las consultas GORM no se corrieron contra una BD.
