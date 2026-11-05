# Notificaciones push: guía para el equipo móvil (Android)

Qué tiene que hacer la app para recibir las alertas de Agatha por push, y cómo probarlo en un celular real.
Cómo funciona el módulo completo: [NOTIFICATIONS_README.MD](NOTIFICATIONS_README.MD), sección "Cómo funciona". El contrato acordado está en
[CONTRATO_PUSH_FCM.md](CONTRATO_PUSH_FCM.md); esta guía lo resume y agrega lo que cambió desde entonces.

## 1. En pocas palabras

1. La app obtiene su token de Firebase Cloud Messaging (FCM) y lo registra en el backend con
   `PUT /api/devices/push-token`.
2. Cuando aparece una alerta o sube de nivel, el backend envía un push **de solo datos** a cada celular
   registrado.
3. La app arma la notificación con sus propios textos, descarta los repetidos por `messageId` y, al tocarla,
   abre el detalle de la alerta.

- Proyecto de Firebase: `agatha-5fe71`. La app solo necesita su `google-services.json`; la credencial del
  servidor (cuenta de servicio) **no** se comparte ni hace falta en la app.
- La app ya recibe y muestra push (probado con la consola de Firebase). Lo nuevo está marcado con
  **(nuevo)** en la lista de §2.

## 2. Lista de lo que la app tiene que hacer

- [ ] Obtener el token FCM al arrancar y escuchar `onNewToken` para enterarse cuando cambie.
- [ ] Registrar el token (§3): al obtenerlo, al renovarse y **en cada arranque de la app** **(nuevo)**.
- [ ] Reintentar el registro solo por errores pasajeros; **no** reintentar los `4xx` **(nuevo)** (§3.2).
- [ ] Recibir el mensaje en `onMessageReceived`: como no trae bloque `notification`, siempre llega ahí,
      con la app abierta, en segundo plano o cerrada (§4).
- [ ] Validar el mensaje: sin `alertId` o sin un `level` válido, descartarlo.
- [ ] **Deduplicar por `messageId`** **(nuevo)** (§4.2).
- [ ] Mostrar la notificación con el texto de la app en el idioma del celular (§4.3).
- [ ] Al tocarla, abrir el detalle de la alerta y, si no existe, mostrar "no encontrada" sin cerrarse
      **(nuevo)** (§4.4).
- [ ] Android 13 o más reciente: pedir el permiso `POST_NOTIFICATIONS`; sin él, el push llega pero no se ve.

## 3. Registro del token

```
PUT /api/devices/push-token
Content-Type: application/json

{ "deviceId": "CEL-1A2B3C", "token": "<token FCM>", "platform": "ANDROID" }

→ 204 No Content
```

Se llama a la misma URL base que la app ya usa para la API de Agatha.

| Campo | Regla |
|---|---|
| `deviceId` | Obligatorio, hasta 128 caracteres. El id propio de la app (`CEL-XXXXXX`), el mismo que firma el origen "App móvil" |
| `token` | Obligatorio, hasta 4096 caracteres |
| `platform` | `ANDROID` (no distingue mayúsculas) |

- **Un celular, un token**: registrar un token nuevo reemplaza al anterior del mismo `deviceId`. Si el token
  ya estaba a nombre de otro `deviceId`, pasa al nuevo.
- **Idempotente**: repetir el mismo registro siempre responde `204`. Por eso se puede llamar en cada arranque
  sin problema.
- Los campos de más se ignoran. El cuerpo puede pesar hasta 4 KB.
- No requiere autenticación (riesgo aceptado del piloto).

### 3.1 Respuestas

| HTTP | Cuerpo | Cuándo |
|---|---|---|
| `204` | — | Guardado |
| `400` | `{ "code": "INVALID_INPUT", "message": "..." }` | JSON inválido, cuerpo de más de 4 KB, campo faltante o demasiado largo, `platform` distinta de `ANDROID` |
| `404` | — | El servidor todavía no tiene el módulo de notificaciones habilitado (hoy pasa en el servidor del piloto, ver §6) |
| `405` | — | Método distinto de `PUT` |
| `500` | `{ "code": "INTERNAL_ERROR", "message": "..." }` | No se pudo guardar |

### 3.2 Reintentos

- **Sí reintentar**, con backoff exponencial y tope (por ejemplo 30 s, 1 min, 2 min… hasta 1 h): fallas de
  red, timeouts, `5xx` y `429` (respetando `Retry-After` si viene).
- **No reintentar** ningún otro `4xx` (`400`, `404`, `405`, `413`). Son definitivos: el mismo cuerpo va a
  fallar siempre. Sacar el registro de la cola y dejarlo en el log.
- Lo que se descartó se recupera solo, porque la app vuelve a registrar el token en el siguiente arranque.
  Así, un `404` mientras el servidor no tenga el módulo habilitado no deja a la app reintentando para
  siempre.

## 4. Recepción

### 4.1 Formato del mensaje

El push es **solo datos** (sin bloque `notification`) y con `android.priority = HIGH`. Todos los valores
son **strings**. Los enums no distinguen mayúsculas y las claves que la app no conozca se ignoran.

| Clave | Obligatoria | Valores |
|---|---|---|
| `alertId` | sí | Código de la alerta, p. ej. `ALR-000142`. Es lo que abre `alert_detail/{alertId}` |
| `level` | sí | `YELLOW`, `ORANGE` o `RED`: el nivel actual |
| `kind` | no (por defecto `NEW`) | `NEW`: alerta nueva. `LEVEL_UP`: la alerta subió de nivel |
| `messageId` | no, pero siempre viene en los push de alertas | Id del aviso, número en texto (p. ej. `"9031"`). En los push de prueba es negativo |
| `sensorId` | no | Nombre del dispositivo, p. ej. `Sensor PK34-470` |
| `pk` | no | Abscisa sin "PK", p. ej. `34+470` |
| `event` | no | `LEAK` (fuga) o `MOVEMENT` (movimiento) |
| `startedAt` | no | Inicio de la alerta, RFC 3339 con la hora de Colombia: `2026-10-18T14:32:00-05:00` |
| `classification` | no | `CONFIRMED` si ya se confirmó. Se omite si no está clasificada |

Los campos opcionales se omiten cuando no hay dato. Ejemplo de lo que recibe la app en `data`:

```json
{
  "messageId": "9031",
  "alertId": "ALR-000142",
  "level": "ORANGE",
  "kind": "LEVEL_UP",
  "sensorId": "Sensor PK34-470",
  "pk": "34+470",
  "event": "LEAK",
  "startedAt": "2026-10-18T14:32:00-05:00"
}
```

### 4.2 Push repetidos

Si el backend reintenta un envío que FCM sí había entregado (porque la respuesta se perdió en la red), el
celular puede recibir el mismo push dos veces. La app debe mostrar **una sola notificación por
`messageId`**, por ejemplo guardando los últimos 50 recibidos.

Una subida de nivel es otro aviso: trae otro `messageId` y el mismo `alertId`. Recomendación: usar el
`alertId` como tag o id de la notificación de Android, para que la subida de nivel **reemplace** la
notificación anterior de esa alerta en vez de apilarse (así lo hace la web).

### 4.3 Qué mostrar

- Ejemplo: `kind = LEVEL_UP`, `level = ORANGE` → "La alerta ALR-000142 subió a Naranja", con dispositivo,
  PK, evento e inicio si vienen.
- Los textos los arma la app (español o inglés según el idioma del celular). El backend no manda texto en el
  push.

### 4.4 Al tocar la notificación

Abre `alert_detail/{alertId}`. La alerta puede no existir: es el caso de los push de prueba
(`ALR-PRUEBA-000001`) o el de una alerta que se consulta en otro servidor. La app debe mostrar un estado de
"alerta no encontrada" y no cerrarse.

## 5. Cuándo llega un push (y cuándo no)

**Llega**:
- Cuando se **genera** una alerta en Amarillo, Naranja o Rojo (`kind = NEW`).
- Cada vez que **sube de nivel**, p. ej. Amarillo → Naranja o Naranja → Rojo, o se reactiva por encima de
  su máximo (`kind = LEVEL_UP`).

**No llega**:
- Por nivel normal (verde) ni por "sin comunicación" (gris).
- Por alertas clasificadas como **Falsa alerta**.
- Por horas que se procesaron con atraso: el aviso ya no es actual (la web sí las muestra).
- Cuando la alerta baja de nivel, finaliza o se clasifica: esos cambios solo se ven en la web y en la API.

Si por alguna razón llega uno de estos, la app también puede descartarlo.

## 6. Estado del servidor

| Servidor | Push | Registro de tokens |
|---|---|---|
| Servidor del piloto (VPS) | **Todavía no habilitado** | Responde `404` (por eso importa §3.2) |
| Backend local del equipo web | Funciona, para las pruebas de §7 | Funciona |

Les avisamos cuando quede habilitado en el servidor del piloto. Desde ese momento, la app empieza a
registrarse sola en el siguiente arranque, sin cambiar nada.

## 7. Prueba en un celular real

Para probar sin esperar una alerta real, el backend tiene un comando que envía **un** push con el formato de
§4.1 a un solo celular.

**Qué necesitamos de ustedes**:
1. Una versión de la app que muestre o escriba en el log el **token FCM** y el **`deviceId`**.
2. Enviarnos el token y el `deviceId` de un celular de prueba **por mensaje privado** (no en grupos ni en el
   repositorio): con el token, cualquiera que tenga la credencial del servidor puede enviarle push a ese
   celular.

**Qué hacemos nosotros**:
1. Validar el token contra Firebase sin entregar nada.
2. Enviar push reales a ese celular, coordinados con ustedes, y anotar qué se vio en cada caso:

| # | `level` | `kind` | App | Qué se espera |
|---|---|---|---|---|
| 1 | `YELLOW` | `NEW` | Abierta | Notificación de alerta nueva en Amarillo |
| 2 | `ORANGE` | `LEVEL_UP` | En segundo plano | "Subió a Naranja"; reemplaza la notificación anterior si usan el tag por `alertId` |
| 3 | `RED` | `LEVEL_UP` | Cerrada (deslizada fuera de recientes) | Notificación de Rojo |
| 4 | `RED` | `NEW` | Cualquiera | Al tocarla, el detalle muestra "alerta no encontrada" sin cerrarse |

Todos los push de prueba traen `alertId = ALR-PRUEBA-000001`, `sensorId = Sensor PK34-470`, `pk = 34+470`,
`event = LEAK`, `startedAt` = la hora del envío y un `messageId` negativo distinto en cada envío.

La deduplicación por `messageId` no se puede probar con este comando, porque cada envío trae un id nuevo.
Conviene cubrirla con una prueba unitaria en la app.

**Si no llega**: revisar que la app tenga el permiso de notificaciones, que el celular no tenga ahorro de
batería agresivo para la app (algunos fabricantes cierran las apps en segundo plano) y que el token enviado
sea el actual (cambia al reinstalar o al borrar los datos de la app).

## 8. Preguntas frecuentes

**¿Hace falta login?** No. El celular se identifica con su `deviceId`.

**¿Qué pasa si se desinstala la app?** FCM le avisa al backend que el token ya no existe y el backend lo borra.

**¿Por qué no usan topics de FCM?** Con pocos celulares, un envío por token permite limpiar los tokens
caducados y limitar el envío a ciertos celulares en las pruebas. Si el número de celulares crece mucho, se
evaluará pasar a topics.

**¿Puede llegar un push de una alerta que ya se clasificó?** Sí, si se clasificó como Confirmada y después
subió de nivel: viene con `classification = CONFIRMED`. Las Falsa alerta nunca generan push.
