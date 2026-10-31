# Notifications

Entrega los eventos de Alert Management por dos canales: **web en tiempo real** (SSE: mapa, panel y banner) y **móvil** (push FCM HTTP v1, contrato en `docs/CONTRATO_PUSH_FCM.md`). No decide qué es una alerta ni guarda alertas: eso es de Alert Management. Plan completo y estado en `docs/PLAN.md`.

> En construcción. El canal web ya funciona (`GET /api/notifications/stream`) y la app ya puede registrar su token (`PUT /api/devices/push-token`); el envío por FCM llega en los próximos commits y, mientras tanto, los avisos para móvil no tienen suscripción.

**Para los clientes (web y app)**: endpoints, eventos, reglas para aplicar el estado y reconexión en [NOTIFICATIONS_API.MD](NOTIFICATIONS_API.MD). Este README describe cómo está hecho el módulo por dentro.

> **Foto = estado actual de la alerta.** Cuando este README, el plan o el código dicen "foto" (`alert_changed`), se refieren al estado completo de la alerta justo después de un cambio. No son las evidencias fotográficas de la especificación (RN-09).

## Estructura

```
notifications/
├── domain/
│   ├── entities/      # AlertNotice, AlertChange (vista propia de los eventos), PushMessage, PushDevice + enums del contrato FCM
│   └── policies/      # ChannelPolicy (Strategy): MobilePushPolicy, WebPolicy
├── application/
│   ├── ports/         # NoticeChannel, ChangeChannel, PushTokenRepository, Clock (los implementa infrastructure)
│   └── services/      # NoticeDispatcher: política + canal; PushTokenService: registro de tokens
└── infrastructure/
    ├── events/        # AlertNotificationSubscriber, AlertChangedSubscriber
    ├── http/          # PushTokenHandler (PUT /api/devices/push-token)
    ├── persistence/   # Tabla push_devices: Migrate, modelo, mapper, repositorio GORM y store en memoria (pruebas)
    └── realtime/      # Hub SSE (canal web) + StreamHandler (GET /api/notifications/stream)
```

## Eventos que consume

| Tópico | Suscripción | Canal |
|---|---|---|
| `alert_management.alert_notification` | `notifications.alert_notification.web` | Web: banner, sonido, notificación del navegador |
| `alert_management.alert_notification` | `notifications.alert_notification.push` | Push FCM |
| `alert_management.alert_changed` | `notifications.alert_changed.web` | Web: mapa y panel |

Cada canal es una suscripción propia del bus, con su cola y su worker: si FCM se atrasa, la web no lo nota. La traducción contrato → `AlertNotice`/`AlertChange` y las suscripciones viven en `internal/bootstrap/notifications.go`; este módulo no importa a Alert Management ni al bus (`depguard` `notifications-isolation`).

## Canal web: `GET /api/notifications/stream`

Responde `text/event-stream` y deja la conexión abierta. Sin cuerpo ni autenticación (ver riesgos).

- **Reconexión**: el primer bloque es `retry: 3000`. Para retomar, el navegador manda la cabecera `Last-Event-ID` (reconexión nativa) o el cliente agrega `?last_event_id=<id>` (reconexión manual con un `EventSource` nuevo).
- **Errores**: `503` con `Retry-After: 30` si se llegó al tope de clientes o la plataforma se está apagando.
- **Plazos**: el handler limpia los deadlines de lectura y escritura del servidor, así que el stream no se corta por `ReadTimeout`/`WriteTimeout`. Envía `X-Accel-Buffering: no` para que un proxy no acumule los eventos.
- **Apagado**: el hub corre como `Runner` ("hub SSE de notificaciones"). Al cancelarse cierra los streams, para que `server.Shutdown` no los espere.


`realtime.Hub` implementa `NoticeChannel` y `ChangeChannel`. Publica dos eventos SSE con `data` en JSON (`snake_case`, `null` cuando no hay valor, como la API de alertas):

| Evento SSE | Origen | Uso en el frontend |
|---|---|---|
| `alert.notification` | `AlertNotice` | Banner, sonido y notificación del navegador (deduplicar por `message_id`) |
| `alert.changed` | `AlertChange` (estado actual de la alerta) | Mapa y panel |
| `resync` | El hub | Se perdieron eventos: volver a pedir `/active` y `/pending` |

- **Ids**: `<epoch>-<seq>`. El epoch cambia en cada arranque del proceso.
- **Reenvío**: guarda los últimos 256 eventos. Al reconectar con `Last-Event-ID` (o `?last_event_id=`) reenvía lo que faltó; si el id ya salió del buffer, es de otro epoch o no se entiende, manda `resync`.
- **Heartbeat**: `: ping` cada 25 s (no se guarda ni se reenvía).
- **Clientes lentos**: cada cliente tiene un buffer de 32 frames; si se llena, el hub lo desconecta sin frenar a los demás, y el cliente se resincroniza al volver.
- **Tope**: 200 clientes simultáneos por defecto; se cambia con `NOTIFICATIONS_SSE_MAX_CLIENTS`.
- Al apagarse (`Run` termina), cierra todos los clientes.

## Reglas por canal

| Aviso | Push (`MobilePushPolicy`) | Web (`WebPolicy`) |
|---|---|---|
| Alerta nueva o subida en Amarillo, Naranja o Rojo | Sí | Sí |
| Hora atrasada (`late`) | No | Sí, marcada "Atrasado" |
| Falsa alerta | No | Sí |
| Nivel normal o código desconocido | No | Sí |

Mapeo al push (`entities.NewPushMessage`): `alertId` = código de la alerta (`ALR-000142`); niveles `AMARILLO/NARANJA/ROJO` → `YELLOW/ORANGE/RED`; `FUGA/MOVIMIENTO` → `LEAK/MOVEMENT`; `OPEN` y `NEW_AFTER_CLASSIFIED` → `NEW`; subidas y reactivación → `LEVEL_UP`; `pk` sale de `devices.name` (`PK34-470` → `34+470`) o se omite.

## Tokens de los celulares: `push_devices`

Un celular (`device_id`, el `CEL-XXXXXX` de la app) tiene un solo token FCM vigente, y un token es de un solo celular.

| Columna | Regla |
|---|---|
| `device_id` | Clave primaria, 1 a 128 caracteres |
| `token` | 1 a 4096 caracteres. Único por `md5(token)`: un índice btree directo no admite entradas tan largas |
| `platform` | Solo `ANDROID` |
| `created_at`, `updated_at` | `updated_at` cambia en cada registro |

- **Registro (upsert)**: `PUT /api/devices/push-token` → `204` (detalle en [NOTIFICATIONS_API.MD](NOTIFICATIONS_API.MD)). Un token nuevo reemplaza al anterior del mismo celular; si el token era de otro `device_id`, pasa a este. Es idempotente, porque la app reintenta hasta recibir 2xx.
- **Limpieza**: cuando FCM responde `UNREGISTERED` o `404`, se borra ese token. Si el celular ya registró otro, no se toca.
- **Migración**: `persistence.Migrate` corre al arrancar el módulo. Es idempotente y solo aditiva (`CREATE TABLE IF NOT EXISTS` y un índice), porque corre contra la base del piloto. No usa `AutoMigrate`.

## Habilitación

`notifications` en `AGATHA_MODULES`. No exige `alert_management`: sin él no recibe eventos, y así se puede levantar solo en local sin la cadena de schedulers que procesa la base del piloto.

## Variables de entorno

| Variable | Por defecto | Uso |
|---|---|---|
| `NOTIFICATIONS_SSE_MAX_CLIENTS` | `200` | Tope de streams abiertos a la vez (un valor inválido usa el de por defecto) |

## Riesgos conocidos

- **Stream sin autenticación** (aceptado para el piloto): cualquiera que llegue al backend puede abrirlo. El tope de clientes limita el abuso.
- **Registro de tokens sin autenticación** (aceptado para el piloto): cualquiera podría registrar un token y recibir los push. Lo acotan el cuerpo de 4 KB, `token` ≤ 4096, `deviceId` ≤ 128 y `platform = ANDROID`.
- Con HTTP/1.1 el navegador abre como máximo 6 conexiones por origen, y cada pestaña ocupa una con el stream.

- El bus es en memoria: un aviso se pierde si el proceso cae entre el commit y la publicación. La fuente de verdad es `alert_messages` (`kind = NOTIFICACION`).
- Mejora futura: la abscisa debería ser una columna propia del dispositivo, no deducirse del nombre.
