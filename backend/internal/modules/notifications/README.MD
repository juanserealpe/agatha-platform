# Notifications

Entrega los eventos de Alert Management por dos canales: **web en tiempo real** (SSE: mapa, panel y banner) y **móvil** (push FCM HTTP v1, contrato en `docs/CONTRATO_PUSH_FCM.md`). No decide qué es una alerta ni guarda alertas: eso es de Alert Management. Plan completo y estado en `docs/PLAN.md`.

> En construcción. El canal web ya funciona (`GET /api/notifications/stream`) y la app ya puede registrar su token (`PUT /api/devices/push-token`); y el canal móvil envía por FCM cuando hay credencial. Para desarrollo hay un endpoint que simula eventos de alerta.

**Para los clientes (web y app)**: endpoints, eventos, reglas para aplicar el estado y reconexión en [NOTIFICATIONS_API.MD](NOTIFICATIONS_API.MD). Este README describe cómo está hecho el módulo por dentro.

> **Foto = estado actual de la alerta.** Cuando este README, el plan o el código dicen "foto" (`alert_changed`), se refieren al estado completo de la alerta justo después de un cambio. No son las evidencias fotográficas de la especificación (RN-09).

## Estructura

```
notifications/
├── domain/
│   ├── entities/      # AlertNotice, AlertChange (vista propia de los eventos), PushMessage, PushDevice + enums del contrato FCM
│   └── policies/      # ChannelPolicy (Strategy): MobilePushPolicy, WebPolicy
├── application/
│   ├── ports/         # NoticeChannel, ChangeChannel, PushTokenRepository, PushSender (+ errores), Clock
│   └── services/      # NoticeDispatcher: política + canal; PushTokenService: registro; MobilePushService: canal móvil
└── infrastructure/
    ├── events/        # AlertNotificationSubscriber, AlertChangedSubscriber
    ├── fcm/           # HTTPV1Sender (Adapter de FCM HTTP v1 + OAuth2 de la cuenta de servicio), NoopSender
    │   └── fcmtest/   # FakeSender: graba los envíos y devuelve errores programados (pruebas)
    ├── http/          # PushTokenHandler (PUT /api/devices/push-token), TestEventHandler (POST /api/notifications/test-event)
    ├── persistence/   # Tabla push_devices (Migrate, modelo, mapper, repositorio GORM, store en memoria) y lector de devices
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
- **Heartbeat**: evento `ping` cada 25 s, sin id (no se guarda ni se reenvía). No es un comentario `: ping` porque `EventSource` no entrega los comentarios a JavaScript y el watchdog del cliente no los vería.
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

## Envío por FCM (`infrastructure/fcm`)

`HTTPV1Sender` implementa `ports.PushSender`: un `POST` a `https://fcm.googleapis.com/v1/projects/<id>/messages:send` por token.

- **Autenticación**: OAuth2 con la cuenta de servicio (`golang.org/x/oauth2/google`, permiso `firebase.messaging`). El token de acceso se pide en el primer envío y se reutiliza hasta que vence. No se usa el Firebase Admin SDK ni la "server key" legacy.
- **Credencial**: archivo montado (`FCM_SERVICE_ACCOUNT_FILE`) o, de respaldo, base64 (`FCM_SERVICE_ACCOUNT_JSON_BASE64`); si están los dos, gana el archivo. Solo se acepta el tipo `service_account`. Ni el JSON ni partes de él van al log o a los errores.
- **Mensaje** (contrato §3): solo datos, `android.priority = HIGH`, valores string, opcionales vacíos omitidos y `startedAt` con offset de Colombia (`-05:00`). Con `ValidateOnly`, FCM valida sin entregar.
- **Cada envío tiene un plazo de 10 s**, incluido pedir el token.

Cómo clasifica las respuestas (los reintentos y la limpieza los hace el servicio del canal móvil):

| Respuesta | Error devuelto | Qué significa |
|---|---|---|
| `200` | ninguno | Entregado (o validado) |
| `errorCode = UNREGISTERED` | `ports.ErrPushTokenUnregistered` | Borrar el token |
| `429`, `5xx`, falla de red | `*ports.RetryablePushError` (con `Retry-After` si vino) | Reintentar |
| Google rechaza la credencial, `400`, `401`, `403`, `404` sin `UNREGISTERED` | error definitivo | No reintentar; revisar configuración |

**Un `404` sin `UNREGISTERED` no borra el token.** FCM también responde `404` si el id del proyecto está mal, y borrar por eso dejaría sin push a todos los celulares del piloto.

Sin credencial, o si la credencial no sirve, se usa `NoopSender`: no envía y lo deja en el log. La plataforma arranca igual y el canal web no depende del push.

## Canal móvil (`MobilePushService`)

Implementa `NoticeChannel` para la suscripción `notifications.alert_notification.push`, detrás de `MobilePushPolicy`. Por cada aviso:

1. Lee `devices.name` del punto (solo lectura) para `sensorId` y `pk`. Si falla, envía sin ellos.
2. Arma el `PushMessage` y lista los celulares de `push_devices`, filtrados por la allowlist si está definida.
3. Envía a cada celular, hasta 4 en paralelo. Un celular que falla no frena a los demás.
4. **Reintentos**: solo los errores reintentables (`429`, `5xx`, red). Hasta 4 intentos, con esperas de 1 s, 2 s y 4 s, o lo que pida `Retry-After` si es mayor (tope de 30 s).
5. **Limpieza**: `UNREGISTERED` borra ese token y no cuenta como falla. Un error definitivo no borra nada.
6. Deja un resumen en el log (entregados, tokens borrados, fallidos). Si alguno falló, devuelve error y el despachador lo registra. Los tokens nunca van al log: se identifican por `deviceId`.

La suscripción del push tiene un plazo de 3 min (las demás, 30 s): alcanza para los reintentos de los celulares del piloto. Si se acaba, los reintentos pendientes se cortan y quedan en el log.

## Eventos de prueba (`POST /api/notifications/test-event`)

El handler valida el pedido (`entities.NewTestEvent`) y llama al puerto `TestEventEmitter`. Lo implementa `internal/bootstrap` (`busTestEventEmitter`), que publica los mismos contratos de Alert Management en el bus. Así el evento recorre el pipeline real sin que este módulo importe esos contratos y sin escribir en la base. Los ids son negativos y el código es `ALR-PRUEBA-…`, para que nunca se confundan con una alerta real. Uso y ejemplos en [NOTIFICATIONS_API.MD](NOTIFICATIONS_API.MD) §3.

## Habilitación

`notifications` en `AGATHA_MODULES`. No exige `alert_management`: sin él no recibe eventos, y así se puede levantar solo en local sin la cadena de schedulers que procesa la base del piloto.

## Variables de entorno

Plantilla con valores seguros por defecto en [`backend/.env.example`](../../../.env.example).

| Variable | Por defecto | Uso |
|---|---|---|
| `NOTIFICATIONS_SSE_MAX_CLIENTS` | `200` | Tope de streams abiertos a la vez (un valor inválido usa el de por defecto) |
| `FCM_PROJECT_ID` | `agatha-5fe71` | Proyecto Firebase |
| `FCM_SERVICE_ACCOUNT_FILE` | — | Ruta del JSON de la cuenta de servicio (montado en solo lectura). Tiene prioridad |
| `FCM_SERVICE_ACCOUNT_JSON_BASE64` | — | Respaldo: el mismo JSON en base64 |
| `FCM_VALIDATE_ONLY` | `true` | FCM valida los mensajes sin entregarlos. Los push reales se piden con `false`, como hace el `docker-compose.yml` del VPS; así un backend local con la credencial no llega a los celulares del piloto |
| `NOTIFICATIONS_PUSH_DEVICE_ALLOWLIST` | — | `deviceId` separados por coma: solo se envía a esos. Vacía = todos |
| `NOTIFICATIONS_TEST_ENDPOINT` | `false` | Habilita `POST /api/notifications/test-event` si `AGATHA_ENV` no es `production`. **Nunca en el VPS** |

## Despliegue (Docker)

- `docker-compose.yml` incluye `notifications` en `AGATHA_MODULES`, define `FCM_VALIDATE_ONLY=false` y fuerza `NOTIFICATIONS_TEST_ENDPOINT=false` (las variables de `environment` pisan las del `.env`).
- **Credencial**: va en `backend/secrets/fcm-service-account.json` del VPS, que se monta en solo lectura en `/run/secrets/agatha/`. Git y `.dockerignore` ignoran `secrets/`, así que nunca llega al repositorio ni a la imagen. El contenedor corre con un usuario sin root, así que el archivo tiene que ser legible (`chmod 444`).
- Si falta la credencial, el push queda deshabilitado (`NoopSender`, con el error en el log) y el resto de la plataforma arranca igual.

## Riesgos conocidos

- **Stream sin autenticación** (aceptado para el piloto): cualquiera que llegue al backend puede abrirlo. El tope de clientes limita el abuso.
- **Registro de tokens sin autenticación** (aceptado para el piloto): cualquiera podría registrar un token y recibir los push. Lo acotan el cuerpo de 4 KB, `token` ≤ 4096, `deviceId` ≤ 128 y `platform = ANDROID`.
- Con HTTP/1.1 el navegador abre como máximo 6 conexiones por origen, y cada pestaña ocupa una con el stream.

- El bus es en memoria: un aviso se pierde si el proceso cae entre el commit y la publicación. La fuente de verdad es `alert_messages` (`kind = NOTIFICACION`).
- Mejora futura: la abscisa debería ser una columna propia del dispositivo, no deducirse del nombre.
