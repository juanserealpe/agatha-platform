# Contrato de push de alertas (FCM) — nota para el equipo web/backend

HU-4.1 (Jira AGT-25). La app Android ya recibe y muestra push de alertas con Firebase Cloud
Messaging (probado en un celular real con un mensaje de prueba de la consola de Firebase).
Código de referencia en la app: `push/AlertPush.kt`, `push/AgathaMessagingService.kt`,
`push/PushTokenStore.kt`, `core/device/DeviceOriginProvider.kt`.

**Estado (2026-10-31): implementado en el backend** (módulo `notifications`, detalle en
[NOTIFICATIONS_README.MD](NOTIFICATIONS_README.MD)). Este documento es el contrato final:
lo que antes era propuesta quedó como se describe aquí.

## 1. Registro del token (implementado en el backend)

La app no tiene login: cada celular se identifica con un `deviceId` propio (`CEL-XXXXXX`,
el mismo que firma el origen "App móvil" de sus registros, RN-10):

```
PUT /api/devices/push-token
{ "deviceId": "CEL-1A2B3C", "token": "<token FCM>", "platform": "ANDROID" }
→ 204 No Content
```

- **Upsert por `deviceId`**: un celular tiene un solo token vigente; un token nuevo
  reemplaza al anterior. Es idempotente: repetir el mismo registro siempre da `204`.
- **Cuándo reintentar** (acordado por el backend; pedimos ajustar la cola de la app):
  - **Sí**, con backoff exponencial y tope (p. ej. 30 s, 1 min, 2 min… hasta 1 h): fallas
    de red, timeouts, `5xx` y `429` (respetando `Retry-After` si viene).
  - **No**: cualquier otro `4xx` (`400`, `404`, `405`, `413`). Son definitivos: el mismo
    cuerpo va a fallar siempre. Sacar el registro de la cola y dejarlo en el log. Se
    vuelve a intentar solo cuando cambie algo (token nuevo, actualización de la app).
  - Así un dato inválido no deja a la app reintentando para siempre ni cargando al servidor.
- **Volver a registrar en cada arranque de la app** (además de al obtener o renovar el
  token). Es idempotente y barato, y recupera los registros que se descartaron por un `4xx`:
  por ejemplo, un `404` mientras el backend desplegado todavía no tenga el módulo de
  notificaciones habilitado, o un token que el backend borró.
- **Respuestas**: `204` guardado; `400 INVALID_INPUT` con `{ "code", "message" }` si el JSON
  es inválido, pasa de 4 KB, falta un campo, `deviceId` supera 128 caracteres, `token`
  supera 4096 o `platform` no es `ANDROID` (sin distinguir mayúsculas); `500` si no se
  pudo guardar. Los campos extra se ignoran.
- La app lo llama al obtener o renovar el token. Ruta, verbo y nombres quedaron como se
  propusieron.

## 2. Envío

- **API FCM HTTP v1** (`POST https://fcm.googleapis.com/v1/projects/agatha-5fe71/messages:send`),
  autenticada con la **cuenta de servicio** del proyecto Firebase (OAuth2, sin el Admin SDK).
  No se usa la API legacy ni la "server key".
- El JSON de la cuenta de servicio es un secreto: en el VPS es un archivo montado en solo
  lectura, **nunca** va al repositorio, a la imagen, a un chat o a un correo.
- Un mensaje por celular registrado. Si FCM falla por algo pasajero (`429`, `5xx`, red), se
  reintenta hasta 4 veces en total, respetando `Retry-After`. La app no tiene que hacer nada.

## 3. Formato del mensaje

- **Solo datos**: sin bloque `notification`. La app arma el texto con sus propios recursos
  (español/inglés según el idioma del celular) y así el mensaje llega también con la app en
  segundo plano o cerrada.
- `android.priority: "HIGH"`.
- Todos los valores de `data` son **strings**. Campos extra se ignoran; enums sin
  distinguir mayúsculas.

| Clave | Obligatoria | Valores |
|---|---|---|
| `alertId` | sí | código de la alerta, `ALR-000142` (abre `alert_detail/{alertId}` al tocar) |
| `level` | sí | `YELLOW` / `ORANGE` / `RED` (nivel actual) |
| `kind` | no (por defecto `NEW`) | `NEW` (alerta generada) / `LEVEL_UP` (subió de nivel) |
| `sensorId` | no | nombre del dispositivo (`devices.name`, p. ej. `Sensor PK34-470`) |
| `pk` | no | abscisa sin el prefijo "PK" (p. ej. `34+470`), sacada del nombre del dispositivo |
| `event` | no | `LEAK` / `MOVEMENT`. El backend hoy no envía `LEAK_AND_MOVEMENT` |
| `startedAt` | no | inicio de la alerta, RFC 3339 con offset de Colombia (`2026-10-18T14:32:00-05:00`) |
| `classification` | no | `CONFIRMED`; se omite si está Sin clasificar (`FALSE_ALARM` nunca se envía, §4) |
| `messageId` | no | id del aviso (número en texto, p. ej. `"9031"`). Los push de alertas siempre lo traen |

**Push repetidos**: si el backend reintenta un envío que FCM sí había entregado (respuesta
perdida por la red), el celular puede recibir el mismo push dos veces. La app debe
mostrar una sola notificación por `messageId` (por ejemplo, guardando los últimos 50
recibidos). Una subida de nivel es otro aviso, con otro `messageId` y el mismo `alertId`.

Los campos opcionales se omiten cuando no hay dato (p. ej. un nombre de dispositivo sin PK
no lleva `pk`). Un mensaje sin `alertId` o sin `level` válido se descarta en la app.

## 4. Cuándo enviar (HU-4.1)

- **Sí:** al **generarse** una alerta (`kind=NEW`) y cada vez que **sube de nivel**
  (`kind=LEVEL_UP`, p. ej. Amarillo → Naranja), a todos los tokens registrados.
- **Nunca:** por avisos **grises** (sin comunicación / indicador no calculable, RN-02), por
  el nivel **normal** (Verde), ni por alertas clasificadas o suprimidas como **Falsa alerta**.
  La app también los descarta si llegan, pero el backend no los envía.
- **Tampoco** por horas que llegan atrasadas (se procesan tarde): el aviso ya no es actual.
  La web sí las muestra, marcadas como atrasadas.
- Mensajes de Alert Management que generan push: `OPEN` y `NEW_AFTER_CLASSIFIED` → `NEW`;
  `UP_NARANJA_PERSIST`, `UP_NARANJA_FAST`, `UP_ROJO` y `REACTIVATED` → `LEVEL_UP`.

## 5. Limpieza de tokens

Si FCM responde `UNREGISTERED` (o `404 NOT_FOUND`) para un token, borrarlo: la app se
desinstaló o el token caducó. Cuando la app obtenga uno nuevo lo volverá a registrar (§1).

## 6. Ejemplo (cuerpo de `messages:send`)

```json
{
  "message": {
    "token": "<token FCM del celular>",
    "android": { "priority": "HIGH" },
    "data": {
      "messageId": "9031",
      "alertId": "ALR-000142",
      "level": "ORANGE",
      "kind": "LEVEL_UP",
      "sensorId": "Sensor PK34-470",
      "pk": "34+470",
      "event": "LEAK",
      "startedAt": "2026-10-18T14:32:00-05:00"
    }
  }
}
```

Resultado en el celular: "La alerta ALR-000142 subió a Naranja", con dispositivo, PK, evento e
inicio; al tocarla abre el detalle de la alerta.

## 7. Push de prueba

Para probar la entrega en un celular sin esperar una alerta real, el backend tiene un
comando que manda **un** push con el formato de §3 a un solo celular (desde `backend/`):

```
go run ./cmd/notifications -test-send -token <token FCM> [-level RED] [-kind NEW]
go run ./cmd/notifications -test-send -device CEL-1A2B3C [-level ORANGE] [-kind LEVEL_UP]
```

- `-token`: el token que la app obtiene de Firebase. No necesita que la app haya llegado al
  backend.
- `-device`: el `deviceId` que la app registró con `PUT /api/devices/push-token` (§1).
- El push trae `alertId = ALR-PRUEBA-000001`, `sensorId = Sensor PK34-470`, `pk = 34+470`,
  `event = LEAK`, `startedAt` = la hora del envío y un `messageId` **negativo** distinto en
  cada envío. Esa alerta no existe: al tocar la notificación, el detalle no la encuentra.
  La app debe mostrar un estado de "no encontrada" sin cerrarse.
- Por seguridad, el backend local solo **valida** con FCM (`FCM_VALIDATE_ONLY=true`): si
  responde que validó, el token y el mensaje son correctos. Para que el push llegue al
  celular se corre con `FCM_VALIDATE_ONLY=false`.
