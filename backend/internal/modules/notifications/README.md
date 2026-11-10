# Notifications

Entrega los eventos de Alert Management por dos canales: **web en tiempo real** (SSE) y **móvil** (push FCM HTTP v1). No decide qué es una alerta ni guarda alertas: eso es de Alert Management, y su API (`/api/alerts/active`, `/api/alerts/pending`, en [ALERT_MANAGEMENT_API.MD](../alert_management/ALERT_MANAGEMENT_API.MD)) sigue siendo la fuente de verdad.

> **"Foto" = estado actual de la alerta.** Cuando el código dice "foto" (`alert_changed`), es el estado completo de la alerta justo después de un cambio. No son las evidencias fotográficas de la especificación (RN-09).

## 1. Cómo funciona

```
Processing ──► Rule Engine ──► Alert Management ──(bus de eventos interno)──┐
                               (guarda la alerta                            ▼
                                y sus mensajes)            ┌──── Notifications ────┐
                                                  Canal web (SSE)          Canal móvil (FCM)
                                                  GET /api/notifications/  un push por cada
                                                  stream                   token en push_devices
```

1. Alert Management guarda el cambio y, **después del commit**, publica dos eventos en el bus:
   - `alert_changed`: la foto de la alerta tras cualquier cambio.
   - `alert_notification`: un aviso para una persona (alerta nueva, subida de nivel, reactivación), con título y texto ya redactados.
2. Cada canal tiene su propia suscripción, cola y worker: si FCM se demora, la web no se entera.
3. Cada canal aplica su política (`ChannelPolicy`, patrón Strategy) y entrega.

### Suscripciones

| Tópico | Suscripción | Canal | Plazo |
|---|---|---|---|
| `alert_management.alert_notification` | `notifications.alert_notification.web` | SSE (`alert.notification`) | 30 s |
| `alert_management.alert_notification` | `notifications.alert_notification.push` | Push FCM | 3 min (alcanza para los reintentos) |
| `alert_management.alert_changed` | `notifications.alert_changed.web` | SSE (`alert.changed`) | 30 s |

La traducción contrato → `AlertNotice`/`AlertChange` y el registro de las suscripciones viven en `internal/bootstrap/notifications.go`. El módulo **no importa** a Alert Management ni al bus (`depguard` `notifications-isolation`).

### Políticas por canal

| Aviso | `WebPolicy` | `MobilePushPolicy` |
|---|---|---|
| Alerta nueva o subida en Amarillo, Naranja o Rojo | Sí | Sí |
| Hora atrasada (`late`) | Sí | No: el aviso ya no es actual |
| Falsa alerta | Sí | No |
| Nivel normal, sin comunicación o código desconocido | Sí | No |

Los cambios silenciosos (baja de nivel, finaliza, se clasifica, se etiqueta) solo viajan como `alert_changed`, que va únicamente a la web.

## 2. Estructura

```
notifications/
├── domain/
│   ├── entities/      # AlertNotice, AlertChange (vista propia de los eventos), PushMessage, PushDevice, TestEvent + enums del contrato FCM
│   └── policies/      # ChannelPolicy (Strategy): MobilePushPolicy, WebPolicy
├── application/
│   ├── ports/         # NoticeChannel, ChangeChannel, PushTokenRepository, PushSender (+ errores), Clock, TestEventEmitter
│   └── services/      # NoticeDispatcher (política + canal), PushTokenService (registro), MobilePushService (canal móvil)
└── infrastructure/
    ├── events/        # AlertNotificationSubscriber, AlertChangedSubscriber
    ├── fcm/           # HTTPV1Sender (FCM HTTP v1 + OAuth2), NoopSender; fcmtest/FakeSender para pruebas
    ├── http/          # PushTokenHandler, TestEventHandler
    ├── persistence/   # Tabla push_devices (Migrate, modelo, mapper, repo GORM, store en memoria) y lector de devices
    └── realtime/      # Hub SSE + StreamHandler
```

## 3. Endpoints

| Endpoint | Para qué |
|---|---|
| `GET /api/notifications/stream` | Stream SSE de alertas (§4) |
| `PUT /api/devices/push-token` | Registro del token FCM de un celular (§5.1) |
| `POST /api/notifications/test-event` | Evento inventado, solo desarrollo (§6.1) |

- **Sin autenticación** (riesgo aceptado del piloto, §8). Los cuerpos tienen tope de tamaño.
- Si `notifications` no está en `AGATHA_MODULES`, responden `404`.
- Errores REST: `{ "code": "...", "message": "..." }`, `message` en español.

## 4. Canal web: `GET /api/notifications/stream`

`realtime.Hub` implementa `NoticeChannel` y `ChangeChannel`; `StreamHandler` responde `text/event-stream` y deja la conexión abierta.

### 4.1 Eventos

| Evento SSE | Origen | Significado |
|---|---|---|
| `alert.changed` | `AlertChange` | Foto de la alerta tras un cambio |
| `alert.notification` | `AlertNotice` | Aviso para mostrar (deduplicable por `message_id`) |
| `resync` | El hub | Se perdieron eventos: el cliente tiene que volver a consultar `/active` y `/pending` |
| `ping` | El hub | Heartbeat cada 25 s, sin `id`, `data: {}`. Es un evento y no un comentario `: ping` porque `EventSource` no entrega comentarios a JavaScript |

`data` en JSON `snake_case`, `null` cuando no hay valor, fechas RFC 3339 en UTC (como la API de alertas).

**`alert.changed`**: `alert_id`, `alert_code` (`ALR-000142`), `measuring_point_id`, `event_type` (`FUGA`, `MOVIMIENTO`), `change` (`OPENED`, `LEVEL_RAISED`, `LEVEL_LOWERED`, `REACTIVATED`, `FINISHED`, `NO_DATA_STARTED`, `NO_DATA_ENDED`, `CLASSIFIED`, `LABELED`), `state` (`ACTIVA`, `FINALIZADA`), `current_level` (`AMARILLO`/`NARANJA`/`ROJO` o `null`), `max_level`, `blinking` (Rojo activo sin clasificar), `management_status` (`SIN_CLASIFICAR` → `CLASIFICADA_SIN_ETIQUETA` → `CLASIFICADA_CON_ETIQUETA`), `classification` (`CONFIRMADA`, `FALSA_ALERTA` o `null`), `started_at`, `evaluated_at` (hora de procesamiento que produjo la foto; sirve para ordenar), `occurred_at`.

**`alert.notification`**: `message_id`, `alert_id`, `alert_code`, `measuring_point_id`, `event_type`, `code` (`OPEN`, `NEW_AFTER_CLASSIFIED`, `UP_NARANJA_PERSIST`, `UP_NARANJA_FAST`, `UP_ROJO`, `REACTIVATED`), `level`, `max_level`, `classification`, `title`, `body` (texto del catálogo de Alert Management), `late`, `started_at`, `evaluated_at`, `occurred_at`. Una subida de nivel conserva el `alert_id` y trae otro `message_id`.

### 4.2 Comportamiento del hub

- **Ids** `<epoch>-<seq>`; el epoch son los milisegundos del arranque del proceso.
- **Reconexión**: el primer bloque es `retry: 3000`. Para retomar, el cliente manda `Last-Event-ID` (reconexión nativa) o `?last_event_id=<id>` (un `EventSource` nuevo no manda la cabecera; además el parámetro no depende de CORS).
- **Reenvío**: guarda los últimos 256 eventos y reenvía lo que faltó. Si el id ya salió del buffer, es de otro epoch o no se entiende, manda `resync`.
- **Clientes lentos**: buffer de 32 frames por cliente; si se llena, lo desconecta sin frenar a los demás (al volver recibe `resync`).
- **Tope**: 200 clientes (`NOTIFICATIONS_SSE_MAX_CLIENTS`). Pasado el tope, o durante el apagado: `503` con `Retry-After: 30`.
- **Plazos**: el handler limpia los deadlines de lectura/escritura del servidor (el stream no se corta por `ReadTimeout`/`WriteTimeout`) y envía `X-Accel-Buffering: no` para que un proxy no acumule eventos.
- **Apagado**: el hub corre como `Runner` ("hub SSE de notificaciones"); al cancelarse cierra todos los streams para que `server.Shutdown` no los espere.
- CORS: el origen tiene que estar en `AGATHA_CORS_ALLOWED_ORIGINS`; CORS ya permite la cabecera `Last-Event-ID`.

## 5. Canal móvil (push FCM)

### 5.1 Registro del token: `PUT /api/devices/push-token`

```
PUT /api/devices/push-token
Content-Type: application/json

{ "deviceId": "CEL-1A2B3C", "token": "<token FCM>", "platform": "ANDROID" }

→ 204 No Content
```

| Campo | Regla |
|---|---|
| `deviceId` | Obligatorio, hasta 128 caracteres (`CEL-XXXXXX`, el id propio de la app; no hay login) |
| `token` | Obligatorio, hasta 4096 caracteres |
| `platform` | `ANDROID` (sin distinguir mayúsculas) |

- **Upsert idempotente** (`PushTokenService`): un celular, un token; uno nuevo reemplaza al anterior. Si el token era de otro `deviceId`, pasa a este. Repetir siempre da `204`, porque la app reintenta hasta recibir 2xx y se vuelve a registrar en cada arranque.
- Campos extra se ignoran; cuerpo hasta 4 KB; se recortan espacios.

| HTTP | `code` | Cuándo |
|---|---|---|
| 204 | — | Guardado |
| 400 | `INVALID_INPUT` | JSON inválido, más de 4 KB, campo faltante o largo, `platform` distinta de `ANDROID` |
| 405 | — | Método distinto de `PUT` |
| 500 | `INTERNAL_ERROR` | No se pudo guardar |

Acordado con el equipo móvil: los `4xx` son definitivos y la app no los reintenta.

### 5.2 Tabla `push_devices`

| Columna | Regla |
|---|---|
| `device_id` | PK, 1 a 128 caracteres |
| `token` | 1 a 4096. Único por `md5(token)`: un índice btree directo no admite entradas tan largas |
| `platform` | Solo `ANDROID` |
| `created_at`, `updated_at` | `updated_at` cambia en cada registro |

`persistence.Migrate` corre al arrancar el módulo: idempotente y solo aditiva (`CREATE TABLE IF NOT EXISTS` + índice), porque corre contra la base del piloto. No usa `AutoMigrate`.

### 5.3 Mensaje (`entities.NewPushMessage`)

Solo datos (sin bloque `notification`, la app arma el texto), `android.priority = HIGH`, todos los valores string, opcionales vacíos omitidos.

| Clave | Obligatoria | Origen |
|---|---|---|
| `alertId` | sí | Código de la alerta (`ALR-000142`) |
| `level` | sí | `AMARILLO/NARANJA/ROJO` → `YELLOW/ORANGE/RED` |
| `kind` | no (def. `NEW`) | `OPEN`, `NEW_AFTER_CLASSIFIED` → `NEW`; `UP_NARANJA_PERSIST`, `UP_NARANJA_FAST`, `UP_ROJO`, `REACTIVATED` → `LEVEL_UP` |
| `messageId` | siempre en push de alertas | Id del aviso, para que la app descarte repetidos |
| `sensorId` | no | `devices.name` (p. ej. `Sensor PK34-470`) |
| `pk` | no | Deducido del nombre: `PK34-470` → `34+470`; se omite si no hay PK |
| `event` | no | `FUGA/MOVIMIENTO` → `LEAK/MOVEMENT` |
| `startedAt` | no | RFC 3339 con offset de Colombia (`-05:00`) |
| `classification` | no | `CONFIRMED`; omitida si sin clasificar (`FALSE_ALARM` nunca sale: la política lo filtra) |

```json
{
  "message": {
    "token": "<token FCM del celular>",
    "android": { "priority": "HIGH" },
    "data": {
      "messageId": "9031", "alertId": "ALR-000142", "level": "ORANGE", "kind": "LEVEL_UP",
      "sensorId": "Sensor PK34-470", "pk": "34+470", "event": "LEAK",
      "startedAt": "2026-10-18T14:32:00-05:00"
    }
  }
}
```

### 5.4 `MobilePushService`

Implementa `NoticeChannel` detrás de `MobilePushPolicy`. Por cada aviso:

1. Lee `devices.name` del punto (solo lectura) para `sensorId` y `pk`; si falla, envía sin ellos.
2. Arma el `PushMessage` y lista `push_devices`, filtrado por `NOTIFICATIONS_PUSH_DEVICE_ALLOWLIST` si está definida.
3. Envía un mensaje por token, hasta 4 en paralelo; un celular que falla no frena a los demás.
4. **Reintentos**, solo en errores reintentables: hasta 4 intentos, esperas de 1 s, 2 s y 4 s, o `Retry-After` si es mayor (tope 30 s).
5. **Limpieza**: `UNREGISTERED` borra ese token (si el celular no registró otro mientras tanto) y no cuenta como falla. Un error definitivo no borra nada.
6. Resumen en el log (entregados, borrados, fallidos); si alguno falló devuelve error y el despachador lo registra. **Los tokens nunca van al log**: se identifican por `deviceId`.

Si un reintento reenvía algo que FCM sí había entregado (respuesta perdida), el celular recibe el push dos veces; por eso viaja `messageId`.

Por qué un envío por token y no topics de FCM: con pocos celulares permite limpiar los caducados y limitar el envío en pruebas (allowlist). Si crece mucho, se evalúa pasar a topics.

### 5.5 `HTTPV1Sender` (`infrastructure/fcm`)

Implementa `ports.PushSender`: `POST https://fcm.googleapis.com/v1/projects/<id>/messages:send`, con OAuth2 de la cuenta de servicio (`golang.org/x/oauth2/google`, permiso `firebase.messaging`). Sin Firebase Admin SDK ni "server key" legacy. El access token se pide en el primer envío y se reutiliza hasta que vence. Plazo de 10 s por envío, incluido pedir el token. Con `ValidateOnly`, FCM valida sin entregar.

| Respuesta | Error devuelto | Qué hace el servicio |
|---|---|---|
| `200` | ninguno | Entregado (o validado) |
| `errorCode = UNREGISTERED` | `ports.ErrPushTokenUnregistered` | Borra el token |
| `429`, `5xx`, falla de red | `*ports.RetryablePushError` (con `Retry-After`) | Reintenta |
| Credencial rechazada, `400`, `401`, `403`, `404` sin `UNREGISTERED` | error definitivo | No reintenta; revisar configuración |

**Un `404` sin `UNREGISTERED` NO borra el token**: FCM también responde `404` si el id del proyecto está mal, y borrar por eso dejaría sin push a todos los celulares del piloto.

**Credencial**: archivo (`FCM_SERVICE_ACCOUNT_FILE`, tiene prioridad) o base64 (`FCM_SERVICE_ACCOUNT_JSON_BASE64`). Solo se acepta el tipo `service_account`; ni el JSON ni partes de él van al log o a los errores. Sin credencial válida se usa `NoopSender` (no envía, lo deja en el log) y la plataforma arranca igual: el canal web no depende del push.

## 6. Herramientas de prueba

### 6.1 `POST /api/notifications/test-event`

`TestEventHandler` valida el pedido (`entities.NewTestEvent`) y llama al puerto `TestEventEmitter`, que implementa `internal/bootstrap` (`busTestEventEmitter`): publica los mismos contratos de Alert Management en el bus. Así el evento recorre el pipeline real (suscripciones, políticas, SSE y push) **sin escribir en la base** y sin que el módulo importe esos contratos.

Solo existe con `NOTIFICATIONS_TEST_ENDPOINT=true` y `AGATHA_ENV` distinto de `production`; si no, `404`.

> **Nunca habilitarlo en el VPS**: el evento llega a todos los streams abiertos y, por push, a todos los celulares registrados. En local, usar `FCM_VALIDATE_ONLY=true` o la allowlist.

| Campo | Obligatorio | Valores |
|---|---|---|
| `measuring_point_id` | sí | `id` de un dispositivo |
| `level` | sí | `AMARILLO`, `NARANJA`, `ROJO` |
| `code` | no (def. `OPEN`) | Código del catálogo (§4.1). Publica `alert_notification` y su `alert_changed` |
| `change` | no | Cambio silencioso: `LEVEL_LOWERED`, `FINISHED`, `NO_DATA_STARTED`, `NO_DATA_ENDED`, `CLASSIFIED`, `LABELED`. Excluyente con `code` |
| `alert_id` | no | Negativo, de un evento anterior: repite sobre la misma alerta. No acepta positivos |
| `event_type` | no (def. `FUGA`) | `FUGA`, `MOVIMIENTO` |
| `classification` | con `CLASSIFIED`/`LABELED` | `CONFIRMADA`, `FALSA_ALERTA` |
| `late` | no | `true`: aviso atrasado (pasa `WebPolicy`, no `MobilePushPolicy`) |

Valores sin distinguir mayúsculas; **cuerpo estricto** (un campo desconocido da `400`). Responde `202` (la entrega es asíncrona) con `{ "alert_id": -1, "alert_code": "ALR-PRUEBA-000001", "message_id": -2, "change": "OPENED" }`. Ids siempre negativos y código `ALR-PRUEBA-…`, para que nunca se confundan con una alerta real.

```bash
curl -X POST localhost:8080/api/notifications/test-event -d '{"measuring_point_id":11556,"level":"AMARILLO"}'
curl -X POST localhost:8080/api/notifications/test-event -d '{"measuring_point_id":11556,"alert_id":-1,"level":"ROJO","code":"UP_ROJO"}'
curl -X POST localhost:8080/api/notifications/test-event -d '{"measuring_point_id":11556,"alert_id":-1,"level":"ROJO","change":"CLASSIFIED","classification":"CONFIRMADA"}'
curl -N localhost:8080/api/notifications/stream   # ver el stream crudo
```

### 6.2 Push de prueba: `cmd/notifications -test-send`

Envía **un** push con el formato de §5.3 a un solo celular, sin esperar una alerta. Lógica en `internal/bootstrap/notification_test_push.go`. Desde `backend/`:

```bash
go run ./cmd/notifications -test-send -token "<token FCM>" [-level RED] [-kind NEW]
go run ./cmd/notifications -test-send -device CEL-1A2B3C [-level ORANGE] [-kind LEVEL_UP]
FCM_VALIDATE_ONLY=false go run ./cmd/notifications -test-send -token "<token FCM>"   # entrega real
```

- `-token` no toca la base; `-device` busca el token en `push_devices` y respeta la allowlist.
- Usa la credencial y `FCM_VALIDATE_ONLY` del `.env`: por defecto FCM **solo valida** el token y el mensaje.
- El push trae `alertId = ALR-PRUEBA-000001`, `sensorId = Sensor PK34-470`, `pk = 34+470`, `event = LEAK`, `startedAt` = hora del envío y un `messageId` negativo distinto en cada envío.
- `UNREGISTERED` = el token ya no sirve. Los tokens de celulares de prueba se comparten **por privado**, nunca en grupos ni en el repo.

### 6.3 Backend local (sin tocar la base del piloto)

```bash
docker run -d --name agatha-pg-local -e POSTGRES_USER=agatha -e POSTGRES_PASSWORD=agatha_local \
  -e POSTGRES_DB=agatha -p 127.0.0.1:5433:5432 postgres:16-alpine   # después: docker start agatha-pg-local
```

`backend/.env` (ignorado por git; **nunca** apuntar `DATABASE_URL` al piloto):

```
DATABASE_URL=postgres://agatha:agatha_local@localhost:5433/agatha?sslmode=disable
AGATHA_ENV=development
AGATHA_MODULES=notifications
FCM_PROJECT_ID=agatha-5fe71
FCM_SERVICE_ACCOUNT_FILE=secrets/fcm-service-account.json
FCM_VALIDATE_ONLY=true
NOTIFICATIONS_TEST_ENDPOINT=true
```

`cd backend && go run ./cmd/platform`; listo cuando el log dice `servidor HTTP iniciado [addr :8080]`. Las tablas se crean al arrancar y sin credencial arranca igual, con el push deshabilitado.

## 7. Configuración y despliegue

Habilitación: `notifications` en `AGATHA_MODULES`. No exige `alert_management` (sin él no recibe eventos), así se puede levantar solo en local sin la cadena de schedulers. Plantilla en [`backend/.env.example`](../../../.env.example).

| Variable | Por defecto | Uso |
|---|---|---|
| `NOTIFICATIONS_SSE_MAX_CLIENTS` | `200` | Tope de streams (inválido → por defecto) |
| `FCM_PROJECT_ID` | `agatha-5fe71` | Proyecto Firebase |
| `FCM_SERVICE_ACCOUNT_FILE` | — | Ruta del JSON de la cuenta de servicio (solo lectura). Prioridad |
| `FCM_SERVICE_ACCOUNT_JSON_BASE64` | — | Respaldo: el mismo JSON en base64 |
| `FCM_VALIDATE_ONLY` | `true` | Valida sin entregar. El compose del VPS pone `false`; así un backend local con la credencial no llega a los celulares del piloto |
| `NOTIFICATIONS_PUSH_DEVICE_ALLOWLIST` | — | `deviceId` separados por coma. Vacía = todos |
| `NOTIFICATIONS_TEST_ENDPOINT` | `false` | Habilita el endpoint de prueba si `AGATHA_ENV` ≠ `production`. **Nunca en el VPS** |

**Docker**: `docker-compose.yml` incluye `notifications` en `AGATHA_MODULES`, define `FCM_VALIDATE_ONLY=false` y fuerza `NOTIFICATIONS_TEST_ENDPOINT=false` y `AGATHA_ENV=production` (las variables de `environment` pisan el `.env`). Son dos bloqueos independientes del endpoint de prueba; `AGATHA_ENV` se compara sin distinguir mayúsculas ni espacios.

**Credencial en el VPS**: `backend/secrets/fcm-service-account.json`, montada en solo lectura en `/run/secrets/agatha/`. Git y `.dockerignore` ignoran `secrets/`, así que nunca llega al repo ni a la imagen. El contenedor corre sin root: el archivo tiene que ser legible (`chmod 444`).

### Pruebas automáticas

Todas sin base de datos (stores en memoria y `fcmtest.FakeSender`), con `go test ./...`:

- **Dominio**: políticas (tabla de casos), extracción del `pk`, armado del `PushMessage`, validación del registro y de los eventos de prueba.
- **Servicios**: `NoticeDispatcher`, `PushTokenService`, `MobilePushService` (reintentos con reloj falso, `Retry-After`, sin reintento en `400`, limpieza por `UNREGISTERED`, allowlist).
- **Infraestructura**: hub SSE (reparto, cliente lento, tope, heartbeat, reenvío con `Last-Event-ID` y `resync`), `StreamHandler`, handlers HTTP, credencial y mensaje de FCM, `HTTPV1Sender` contra `httptest.Server`.
- **Cableado** (`internal/bootstrap`): `notifications_test.go` (traducción de contratos, políticas por canal, bus → SSE) y `notifications_end_to_end_test.go` (Processing → Rule Engine → Alert Management → Notifications con el bus real: el push solo recibe `NEW` y `LEVEL_UP`; la web, todos los avisos y cambios, incluido `CLASSIFIED`).

## 8. Estado y riesgos

| Qué | Estado (2026-11-04) |
|---|---|
| Backend (los dos canales) | Implementado y probado (automático y local) |
| Credencial de FCM | Verificada contra Firebase en modo validación |
| Push a un celular real | Pendiente: falta el token de un celular de prueba (§6.2) |
| Servidor del piloto (VPS) | No habilitado todavía: el stream y `PUT /api/devices/push-token` responden `404` |

- **Stream sin autenticación** (aceptado para el piloto): el tope de clientes limita el abuso.
- **Registro de tokens sin autenticación** (aceptado): cualquiera podría registrar un token y recibir los push. Lo acotan el cuerpo de 4 KB, `token` ≤ 4096, `deviceId` ≤ 128 y `platform = ANDROID`.
- **Bus en memoria**: si el proceso cae entre el commit y la publicación, ese aviso no sale en tiempo real. La alerta sí queda en la base (`alert_messages`, `kind = NOTIFICACION`) y aparece al consultar.
- **HTTP/1.1**: el navegador abre como máximo 6 conexiones por origen y cada pestaña ocupa una con el stream.
- Mejora futura: la abscisa (`pk`) debería ser una columna propia del dispositivo, no deducirse del nombre.
