# API de Notifications: tiempo real y registro de tokens

Lo que necesitan los clientes para recibir las alertas al instante:
- **Web**: abrir el stream SSE, aplicar sus eventos al mapa, al panel y al banner, y reconectarse sin perder nada.
- **App Android**: registrar su token FCM para recibir push.

El módulo no guarda ni consulta alertas. La fuente de verdad sigue siendo la API de Alert Management (`/api/alerts/active` y `/api/alerts/pending`, en [ALERT_MANAGEMENT_API.MD](../alert_management/ALERT_MANAGEMENT_API.MD)). El stream solo avisa que algo cambió y trae el estado nuevo para no tener que volver a pedirlo.

## Resumen de endpoints

| Para qué | Endpoint | Cliente |
|---|---|---|
| Recibir alertas en tiempo real | `GET /api/notifications/stream` | Web |
| Registrar el token FCM del celular | `PUT /api/devices/push-token` | App Android |
| Simular un evento de alerta (solo desarrollo) | `POST /api/notifications/test-event` | Desarrollador (curl) |

## Convenciones

- **Sin autenticación** (riesgo aceptado del piloto). Los cuerpos tienen tope de tamaño.
- **Habilitación**: el backend tiene que tener `notifications` en `AGATHA_MODULES`. Si no está, los dos endpoints responden `404`.
- **Fechas**: RFC 3339 en UTC (`2026-10-30T14:00:00Z`). Convertirlas a hora de Colombia para mostrarlas.
- **Valores vacíos**: `null` (por ejemplo, `classification` sin clasificar), igual que la API de alertas.
- **Errores REST**: `{ "code": "...", "message": "..." }`, con `message` en español.

---

## 1. Stream de alertas: `GET /api/notifications/stream`

Server-Sent Events (`text/event-stream`). Se abre con `EventSource` y la conexión queda abierta.

```ts
const source = new EventSource(`${apiUrl()}/api/notifications/stream`);
source.addEventListener('alert.changed', (e) => applyChange(JSON.parse(e.data)));
source.addEventListener('alert.notification', (e) => showNotice(JSON.parse(e.data)));
source.addEventListener('resync', () => refetchActiveAndPending());
```

`EventSource` no permite cabeceras propias. Con CORS basta que el origen esté permitido, porque es un `GET` simple.

### Eventos

| `event:` | Cuándo llega | Qué hace el frontend |
|---|---|---|
| `alert.changed` | Cualquier cambio visible de una alerta | Actualiza el mapa y el panel **en silencio** |
| `alert.notification` | Un aviso que hay que mostrar: alerta nueva, subida de nivel, reactivación sobre el máximo | Banner, sonido y notificación del navegador |
| `resync` | Se perdieron eventos | Volver a pedir `/api/alerts/active` y `/api/alerts/pending` |

Además, cada 25 s llega un comentario `: ping` (heartbeat). `EventSource` no lo entrega como evento, pero cuenta como actividad para el watchdog (ver "Reconexión").

Un evento en el cable se ve así:

```
id: 1761832800123-42
event: alert.changed
data: {"alert_id":142,"alert_code":"ALR-000142",...}

```

### `alert.changed`: estado actual de la alerta

Es una **foto**: el estado completo de la alerta justo después del cambio. No hace falta consultar la API para aplicarlo.

| Campo | Tipo | Notas |
|---|---|---|
| `alert_id` | número | Id de la alerta |
| `alert_code` | string | `ALR-000142` |
| `measuring_point_id` | número | El `id` del dispositivo (marcador del mapa) |
| `event_type` | string | `FUGA`, `MOVIMIENTO` |
| `change` | string | Qué cambió (tabla siguiente). Sirve para logs y textos; para aplicar el estado basta la foto |
| `state` | string | `ACTIVA`, `FINALIZADA` |
| `current_level` | string \| null | `AMARILLO`, `NARANJA`, `ROJO`. `null` si no hay nivel |
| `max_level` | string | Nivel máximo alcanzado |
| `blinking` | bool | Rojo activo sin clasificar: el marcador titila |
| `management_status` | string | `SIN_CLASIFICAR` → `CLASIFICADA_SIN_ETIQUETA` → `CLASIFICADA_CON_ETIQUETA` |
| `classification` | string \| null | `CONFIRMADA`, `FALSA_ALERTA`; `null` = sin clasificar |
| `started_at` | fecha | Inicio de la alerta |
| `evaluated_at` | fecha | Hora de procesamiento que produjo la foto. **Es la clave para descartar fotos viejas** |
| `occurred_at` | fecha | Cuándo se publicó |

| `change` | Significado |
|---|---|
| `OPENED` | Alerta nueva |
| `LEVEL_RAISED`, `LEVEL_LOWERED` | Subió o bajó de nivel (misma alerta, mismo `alert_id`) |
| `REACTIVATED` | Volvió a `ACTIVA` |
| `FINISHED` | Pasó a `FINALIZADA` |
| `NO_DATA_STARTED`, `NO_DATA_ENDED` | Empezaron o terminaron las horas sin datos |
| `CLASSIFIED`, `LABELED` | Se clasificó o se etiquetó (desde la web o la app) |

**Cómo aplicarla sin retroceder** (los eventos pueden llegar repetidos tras una reconexión):
- Nivel y estado (`state`, `current_level`, `max_level`): solo si `evaluated_at` es **mayor** que el que ya está en caché para esa alerta.
- Gestión (`management_status`, `classification`): solo si `management_status` **avanzó**, porque la gestión nunca retrocede.
- `blinking`: recalcularlo en el cliente como `state = ACTIVA` y `current_level = ROJO` y `management_status = SIN_CLASIFICAR`, para que salga coherente tras mezclar las dos reglas anteriores.
- Una alerta `FINALIZADA` o clasificada sale de "pendientes" según las mismas reglas de `/api/alerts/pending`.

### `alert.notification`: aviso para mostrar

| Campo | Tipo | Notas |
|---|---|---|
| `message_id` | número | **Clave para deduplicar**: banner, sonido y notificación una sola vez por `message_id` |
| `alert_id`, `alert_code`, `measuring_point_id`, `event_type` | | Como en `alert.changed` |
| `code` | string | Código del catálogo: `OPEN`, `NEW_AFTER_CLASSIFIED`, `UP_NARANJA_PERSIST`, `UP_NARANJA_FAST`, `UP_ROJO`, `REACTIVATED` |
| `level` | string | Nivel del aviso |
| `max_level` | string | Nivel máximo de la alerta |
| `classification` | string \| null | Como en `alert.changed` |
| `title`, `body` | string | Texto ya armado en español: se muestra tal cual |
| `late` | bool | La hora llegó atrasada. Se avisa igual, con la etiqueta "Atrasado" |
| `started_at`, `evaluated_at`, `occurred_at` | fecha | |

- Una subida de nivel conserva el `alert_id`: por eso se deduplica por `message_id` y no por alerta.
- **Un aviso por alerta en pantalla**: agrupar por `alert_code` y reemplazar el banner anterior. En la notificación del navegador, `tag = alert_code`.
- El push al celular sigue otras reglas (no envía `late` ni falsas alertas). La web recibe todos los avisos.

### Reconexión y pérdida de eventos

- Cada evento trae `id: <epoch>-<seq>` (el epoch son los milisegundos del arranque del servidor). El servidor guarda los últimos 256. Tratar el id como un texto opaco.
- **Reconexión nativa**: `EventSource` reintenta solo (el servidor sugiere `retry: 3000`) y manda la cabecera `Last-Event-ID`. El servidor reenvía lo que faltó.
- **Reconexión manual**: si `EventSource` queda en `readyState = CLOSED` (por ejemplo, tras un `503`), crear uno nuevo con `?last_event_id=<último id recibido>`, porque uno nuevo no manda la cabecera.
- Si el id ya no está en el buffer, es de otro epoch (el servidor se reinició) o no se entiende, llega `resync` en lugar de los eventos perdidos.
- **Siempre que se (re)conecte, pedir `/active` y `/pending` de nuevo.** El stream complementa la API; no la reemplaza.
- **Watchdog**: si pasan 60 s sin ningún dato (ni el `ping`), cerrar y reconectar a mano.
- Mientras el stream esté conectado no hace falta el polling; activarlo solo cuando esté caído.

### Respuestas de error

| HTTP | Cuándo | Qué hacer |
|---|---|---|
| `503` + `Retry-After: 30` | Tope de clientes alcanzado (200 por defecto) o el servidor se está apagando | Reintentar con backoff; mientras tanto, polling |
| `404` | `notifications` no está habilitado | Quedarse con polling |

### Límites prácticos

- Con HTTP/1.1, el navegador permite 6 conexiones por origen y cada pestaña ocupa una con el stream. Conviene cerrar el stream cuando la pestaña lleva un rato oculta y reabrirlo al volver.
- Un cliente que no lee a tiempo (32 eventos acumulados) se desconecta; al reconectar recibe `resync`.

---

## 2. Registro del token FCM: `PUT /api/devices/push-token`

Lo llama la app Android al obtener o renovar su token. Contrato completo en [docs/CONTRATO_PUSH_FCM.md](../../../../docs/CONTRATO_PUSH_FCM.md).

```
PUT /api/devices/push-token
Content-Type: application/json

{ "deviceId": "CEL-1A2B3C", "token": "<token FCM>", "platform": "ANDROID" }

→ 204 No Content
```

| Campo | Regla |
|---|---|
| `deviceId` | Obligatorio, hasta 128 caracteres. El id propio de la app (`CEL-XXXXXX`) |
| `token` | Obligatorio, hasta 4096 caracteres |
| `platform` | `ANDROID` (no distingue mayúsculas) |

- **Upsert e idempotente**: un celular tiene un solo token; uno nuevo reemplaza al anterior. Repetir el mismo registro siempre da `204`.
- Si el token ya era de otro `deviceId`, pasa al nuevo.
- Los campos extra se ignoran, para que una versión nueva de la app no quede reintentando por un campo que el backend no conoce.
- Cuerpo de hasta 4 KB. Los espacios al inicio y al final de los valores se quitan.
- Cuando FCM avisa que un token ya no sirve (`UNREGISTERED`), el backend lo borra. La app lo vuelve a registrar al obtener uno nuevo.

| HTTP | `code` | Cuándo |
|---|---|---|
| 204 | — | Guardado |
| 400 | `INVALID_INPUT` | JSON inválido, cuerpo de más de 4 KB, campo faltante o demasiado largo, plataforma distinta de `ANDROID` |
| 405 | — | Método distinto de `PUT` |
| 500 | `INTERNAL_ERROR` | No se pudo guardar |

**Reintentos del cliente**: reintentar con backoff exponencial y tope solo ante fallas de red, timeouts, `5xx` y `429` (respetando `Retry-After`). Cualquier otro `4xx` es definitivo: el mismo cuerpo siempre va a fallar, así que se descarta y se registra en el log. Reintentar un `400` dejaría a la app en un bucle infinito.

**Registro en cada arranque**: la app vuelve a llamar este endpoint cada vez que arranca, además de cuando obtiene o renueva el token. Así recupera lo que se haya descartado por un `4xx` (por ejemplo, un `404` mientras el backend no tenga `notifications` habilitado) sin reintentar en bucle.

No está pensado para el navegador: CORS solo permite `GET`.

---

## 3. Eventos de prueba: `POST /api/notifications/test-event` (solo desarrollo)

Publica un evento de alerta **inventado** por el mismo camino que los reales: las suscripciones del bus, las políticas de cada canal, el stream SSE y el push. **No escribe nada en la base.** Sirve para probar el mapa, el banner, el sonido, la notificación del navegador y el push sin esperar una alerta real.

**Solo existe si** el backend arrancó con `NOTIFICATIONS_TEST_ENDPOINT=true` y `AGATHA_ENV` distinto de `production`. Si no, responde `404`.

> **Nunca habilitarlo en el VPS.** El evento llega a los canales reales: a todos los navegadores conectados y, por push, a todos los celulares registrados. En local, usar `FCM_VALIDATE_ONLY=true` o `NOTIFICATIONS_PUSH_DEVICE_ALLOWLIST`.

```
POST /api/notifications/test-event
Content-Type: application/json

{ "measuring_point_id": 11556, "level": "ROJO" }

→ 202 Accepted
{ "alert_id": -1, "alert_code": "ALR-PRUEBA-000001", "message_id": -2, "change": "OPENED" }
```

| Campo | Obligatorio | Valores |
|---|---|---|
| `measuring_point_id` | sí | `id` de un dispositivo (el marcador del mapa) |
| `level` | sí | `AMARILLO`, `NARANJA`, `ROJO` |
| `code` | no (por defecto `OPEN`) | Aviso del catálogo: `OPEN`, `NEW_AFTER_CLASSIFIED`, `UP_NARANJA_PERSIST`, `UP_NARANJA_FAST`, `UP_ROJO`, `REACTIVATED`. Publica `alert.notification` y su `alert.changed` |
| `change` | no | Cambio silencioso, sin aviso: `LEVEL_LOWERED`, `FINISHED`, `NO_DATA_STARTED`, `NO_DATA_ENDED`, `CLASSIFIED`, `LABELED`. Excluyente con `code` |
| `alert_id` | no | Negativo, devuelto por un evento anterior: repite sobre la **misma** alerta. Sin él se crea una nueva |
| `event_type` | no (por defecto `FUGA`) | `FUGA`, `MOVIMIENTO` |
| `classification` | no (obligatoria con `CLASSIFIED`/`LABELED`) | `CONFIRMADA`, `FALSA_ALERTA` |
| `late` | no | `true` para probar un aviso atrasado (va a la web, no al push) |

- Los valores no distinguen mayúsculas. **Cuerpo estricto**: un campo desconocido da `400`, para que se note un error de tipeo.
- **Ids negativos**: `alert_id` y `message_id` siempre son negativos y el código es `ALR-PRUEBA-…`. Así nunca pisan una alerta real en la caché del frontend, y el endpoint no acepta un `alert_id` positivo. Al resincronizar con `/active` y `/pending` desaparecen, porque no existen en la base.
- `202` porque la entrega a cada canal es asíncrona.

Una secuencia típica para probar el frontend:

```bash
curl -X POST localhost:8080/api/notifications/test-event -d '{"measuring_point_id":11556,"level":"AMARILLO"}'
# → alert_id -1: el marcador se pone amarillo, banner "nueva"
curl -X POST localhost:8080/api/notifications/test-event -d '{"measuring_point_id":11556,"alert_id":-1,"level":"ROJO","code":"UP_ROJO"}'
# → misma alerta en rojo y titilando, banner "subió"
curl -X POST localhost:8080/api/notifications/test-event -d '{"measuring_point_id":11556,"alert_id":-1,"level":"ROJO","change":"CLASSIFIED","classification":"CONFIRMADA"}'
# → deja de titilar, sin banner
```

| HTTP | `code` | Cuándo |
|---|---|---|
| 202 | — | Publicado |
| 400 | `INVALID_INPUT` | JSON inválido, campo desconocido o valor fuera de las listas |
| 404 | — | El endpoint no está habilitado |
