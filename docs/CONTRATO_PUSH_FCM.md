# Contrato de push de alertas (FCM) — nota para el equipo web/backend

HU-4.1 (Jira AGT-25). La app Android ya recibe y muestra push de alertas con Firebase Cloud
Messaging (probado en un celular real con un mensaje de prueba de la consola de Firebase).
Falta lo del lado del backend: guardar el token de cada celular y enviar los mensajes.
Código de referencia en la app: `push/AlertPush.kt`, `push/AgathaMessagingService.kt`,
`push/PushTokenStore.kt`, `core/device/DeviceOriginProvider.kt`.

## 1. Registro del token (implementado en el backend)

La app no tiene login: cada celular se identifica con un `deviceId` propio (`CEL-XXXXXX`,
el mismo que firma el origen "App móvil" de sus registros, RN-10). Proponemos:

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
- **Respuestas**: `204` guardado; `400 INVALID_INPUT` con `{ "code", "message" }` si el JSON
  es inválido, pasa de 4 KB, falta un campo, `deviceId` supera 128 caracteres, `token`
  supera 4096 o `platform` no es `ANDROID` (sin distinguir mayúsculas); `500` si no se
  pudo guardar. Los campos extra se ignoran.
- La app lo llama al obtener o renovar el token. Ruta, verbo y nombres quedaron como se
  propusieron.

## 2. Envío

- Usar la **API FCM HTTP v1** (`POST https://fcm.googleapis.com/v1/projects/<id>/messages:send`)
  autenticada con la **cuenta de servicio** del proyecto Firebase (OAuth2, p. ej. con el
  Firebase Admin SDK). **No** usar la API legacy ni la "server key".
- El JSON de la cuenta de servicio es un secreto: va en la configuración/secret manager del
  servidor, **nunca** en un repositorio, chat o correo.

## 3. Formato del mensaje

- **Solo datos**: sin bloque `notification`. La app arma el texto con sus propios recursos
  (español/inglés según el idioma del celular) y así el mensaje llega también con la app en
  segundo plano o cerrada.
- `android.priority: "HIGH"`.
- Todos los valores de `data` son **strings**. Campos extra se ignoran; enums sin
  distinguir mayúsculas.

| Clave | Obligatoria | Valores |
|---|---|---|
| `alertId` | sí | id de la alerta (abre `alert_detail/{alertId}` al tocar) |
| `level` | sí | `YELLOW` / `ORANGE` / `RED` (nivel actual) |
| `kind` | no (por defecto `NEW`) | `NEW` (alerta generada) / `LEVEL_UP` (subió de nivel) |
| `sensorId` | no | dispositivo |
| `pk` | no | abscisa sin el prefijo "PK" (p. ej. `12+350`) |
| `event` | no | `LEAK` / `MOVEMENT` / `LEAK_AND_MOVEMENT` |
| `startedAt` | no | inicio de la alerta, RFC 3339 (`2026-10-18T14:32:00-05:00`) |
| `classification` | no | `CONFIRMED` / `FALSE_ALARM`; omitir si está Sin clasificar |

Un mensaje sin `alertId` o sin `level` válido se descarta en la app.

## 4. Cuándo enviar (HU-4.1)

- **Sí:** al **generarse** una alerta (`kind=NEW`) y cada vez que **sube de nivel**
  (`kind=LEVEL_UP`, p. ej. Amarillo → Naranja), a todos los tokens registrados.
- **Nunca:** por avisos **grises** (sin comunicación / indicador no calculable, RN-02), por
  el nivel **normal** (Verde), ni por alertas clasificadas o suprimidas como **Falsa alerta**.
  La app también los descarta si llegan, pero el backend no debería enviarlos.

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
      "alertId": "ALR-0142",
      "level": "ORANGE",
      "kind": "LEVEL_UP",
      "sensorId": "AGT-S-07",
      "pk": "12+350",
      "event": "LEAK",
      "startedAt": "2026-10-18T14:32:00-05:00"
    }
  }
}
```

Resultado en el celular: "La alerta ALR-0142 subió a Naranja", con dispositivo, PK, evento e
inicio; al tocarla abre el detalle de la alerta.
