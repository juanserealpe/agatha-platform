# Notificaciones en tiempo real: guía para el equipo web

Cómo recibe la web las alertas al instante, qué reglas hay que respetar y dónde está implementado en el
frontend del Centro de Control. Cómo funciona el módulo completo: [NOTIFICATIONS_README.MD](NOTIFICATIONS_README.MD), sección "Cómo funciona".

## 1. En pocas palabras

- La web abre **una** conexión Server-Sent Events (SSE) con `GET /api/notifications/stream` y la deja
  abierta.
- Por ahí llegan dos tipos de evento:
  - `alert.changed`: el estado completo de una alerta tras cualquier cambio. Actualiza el mapa y el panel
    **en silencio**.
  - `alert.notification`: un aviso para el operador. Muestra un banner, suena un tono y, si la pestaña
    está oculta, sale una notificación del sistema.
- El stream **complementa** la API de alertas, no la reemplaza. Al conectar o reconectar se vuelven a pedir
  `GET /api/alerts/active` (puntos del mapa) y `GET /api/alerts/pending` (panel "Sin clasificar"). Si el
  stream cae, se vuelve a consultar cada 30 s.

## 2. Requisitos

| Qué | Dónde |
|---|---|
| El backend tiene `notifications` en `AGATHA_MODULES` | Si no, el stream responde `404` y la web sigue con la consulta cada 30 s |
| El origen de la web está en `AGATHA_CORS_ALLOWED_ORIGINS` | Variable del backend. CORS ya permite la cabecera `Last-Event-ID` |
| URL del backend | `VITE_API_BASE_URL` en el build. En `npm run dev`, Vite hace de proxy |

No requiere autenticación (riesgo aceptado del piloto). `EventSource` no permite agregar cabeceras propias.

## 3. El stream

```ts
const source = new EventSource(`${apiUrl}/api/notifications/stream`);
source.addEventListener("alert.changed", (e) => applyChange(JSON.parse(e.data)));
source.addEventListener("alert.notification", (e) => showNotice(JSON.parse(e.data)));
source.addEventListener("resync", () => refetchActiveAndPending());
source.addEventListener("ping", () => resetWatchdog());
```

| Evento | Cuándo llega | Qué hace la web |
|---|---|---|
| `alert.changed` | Cualquier cambio visible de una alerta | Actualiza el mapa y el panel, sin avisar |
| `alert.notification` | Alerta nueva, subida de nivel o reactivación | Banner, sonido y notificación del navegador |
| `resync` | Se perdieron eventos | Volver a pedir `/active` y `/pending` |
| `ping` | Cada 25 s, sin `id` y con `data: {}` | Solo demuestra que la conexión sigue viva (watchdog, §5) |

Cada evento (salvo `ping`) trae `id: <epoch>-<seq>`. Hay que tratarlo como un texto opaco. Los datos vienen
en JSON `snake_case`, con `null` cuando no hay valor y fechas RFC 3339 en UTC. Para mostrarlas hay que
convertirlas a la hora de Colombia.

### 3.1 `alert.changed`

| Campo | Tipo | Notas |
|---|---|---|
| `alert_id` | número | Id de la alerta |
| `alert_code` | string | `ALR-000142` |
| `measuring_point_id` | número | Id del dispositivo (el marcador del mapa) |
| `event_type` | string | `FUGA`, `MOVIMIENTO` |
| `change` | string | `OPENED`, `LEVEL_RAISED`, `LEVEL_LOWERED`, `REACTIVATED`, `FINISHED`, `NO_DATA_STARTED`, `NO_DATA_ENDED`, `CLASSIFIED`, `LABELED`. Sirve para logs y textos |
| `state` | string | `ACTIVA`, `FINALIZADA` |
| `current_level` | string \| null | `AMARILLO`, `NARANJA`, `ROJO` |
| `max_level` | string | Nivel máximo alcanzado |
| `blinking` | bool | Rojo activo sin clasificar |
| `management_status` | string | `SIN_CLASIFICAR` → `CLASIFICADA_SIN_ETIQUETA` → `CLASIFICADA_CON_ETIQUETA` |
| `classification` | string \| null | `CONFIRMADA`, `FALSA_ALERTA`; `null` = sin clasificar |
| `started_at` | fecha | Inicio de la alerta |
| `evaluated_at` | fecha | Hora de procesamiento que produjo este estado. **Clave para descartar estados viejos** |
| `occurred_at` | fecha | Cuándo se publicó |

### 3.2 `alert.notification`

| Campo | Tipo | Notas |
|---|---|---|
| `message_id` | número | **Clave para deduplicar** |
| `alert_id`, `alert_code`, `measuring_point_id`, `event_type` | | Como en `alert.changed` |
| `code` | string | `OPEN`, `NEW_AFTER_CLASSIFIED`, `UP_NARANJA_PERSIST`, `UP_NARANJA_FAST`, `UP_ROJO`, `REACTIVATED` |
| `level`, `max_level` | string | Nivel del aviso y máximo de la alerta |
| `classification` | string \| null | Como en `alert.changed` |
| `title`, `body` | string | Texto en español ya redactado: se muestra tal cual |
| `late` | bool | La hora llegó atrasada. Se avisa igual, con la etiqueta "Atrasado" |
| `started_at`, `evaluated_at`, `occurred_at` | fecha | |

## 4. Reglas para aplicar los eventos

Después de una reconexión, los eventos pueden llegar repetidos o en otro orden. Para que la pantalla no
retroceda:

1. **Nivel y estado** (`state`, `current_level`, `max_level`): aplicarlos solo si `evaluated_at` es mayor que
   el último aplicado a esa alerta.
2. **Gestión** (`management_status`, `classification`): aplicarla solo si `management_status` avanzó. La
   gestión nunca retrocede.
3. **Titileo**: recalcularlo en el cliente como `ACTIVA` + `ROJO` + `SIN_CLASIFICAR`, para que salga coherente
   después de combinar las dos reglas anteriores.
4. **Mapa** (`/active`): una alerta sale al pasar a `FINALIZADA`.
5. **Panel** (`/pending`): una alerta sale solo al clasificarse. Una `FINALIZADA` sin clasificar sigue ahí.
6. **Alertas que no están en caché**: el evento no trae todo lo de la fila (nombre del dispositivo,
   etiqueta…). Si llega una alerta que la web no tiene, se vuelve a pedir la lista en vez de inventar la
   fila.
7. **Avisos**: banner, sonido y notificación una sola vez por `message_id`. Una subida de nivel conserva el
   `alert_id`, así que **no** se deduplica por alerta.

## 5. Conexión y reconexión

| Caso | Qué hacer |
|---|---|
| Corte breve | `EventSource` reintenta solo (el servidor sugiere `retry: 3000`) y manda `Last-Event-ID`. El servidor reenvía lo que faltó |
| `EventSource` queda en `CLOSED` (p. ej. tras un `503` o un `404`) | Crear uno nuevo con `?last_event_id=<último id>`, con backoff exponencial y jitter hasta 30 s. Uno nuevo no manda la cabecera |
| El servidor ya no tiene esos eventos (más de 256, o se reinició) | Llega `resync`: volver a pedir `/active` y `/pending` |
| 60 s sin ningún evento, ni `ping` | La conexión quedó colgada (equipo suspendido, cambio de red): cerrar y reconectar a mano |
| Cada vez que se conecta | Volver a pedir `/active` y `/pending` |
| Stream caído | Consultar `/active` y `/pending` cada 30 s. Con el stream en vivo, no se consulta |

| Respuesta | Cuándo | Qué hacer |
|---|---|---|
| `503` + `Retry-After: 30` | Se llegó al tope de 200 conexiones o el servidor se está apagando | Reintentar con backoff; mientras tanto, consultar cada 30 s |
| `404` | `notifications` no está habilitado | Quedarse con la consulta cada 30 s |

El servidor desconecta a un cliente que no lee a tiempo (32 eventos acumulados). Al volver, ese cliente
recibe `resync`.

## 6. Experiencia del operador

Así está resuelto en el Centro de Control. Si otra web consume el stream, conviene mantener los mismos
criterios.

**Indicador de conexión** (barra superior): "En vivo", "Conectando", "Reconectando" o "Sin conexión" (tras
3 fallas seguidas). Incluye el botón para silenciar el sonido y el que activa las notificaciones del
navegador.

**Banner**:
- Un banner por alerta (se agrupa por `alert_code`): un aviso nuevo de la misma alerta reemplaza al anterior.
- Hasta 3 a la vez, ordenados por nivel (mayor primero) y luego por el más reciente. El resto se resume como
  "+N".
- Teñido según el nivel. En Rojo sin clasificar titila; deja de titilar cuando la alerta se clasifica, aquí,
  en otra pestaña o desde la app.
- Los avisos atrasados llevan la etiqueta "Atrasado".
- Se mantiene al cambiar de sección: un aviso que llega mientras el operador está en Historial sigue visible
  al volver al mapa.

**Sonido**:
- Tono generado con la Web Audio API, sin archivos de audio. Más notas y más agudas cuanto mayor es el
  nivel.
- El navegador bloquea el audio hasta la primera interacción con la página (clic o tecla). Antes de eso no
  suena.
- Con varias pestañas abiertas suena **una sola**: la primera que toma el lock del `message_id` (Web Locks
  API).
- El silencio se guarda en el navegador (`localStorage`).

**Notificación del navegador**:
- El permiso se pide **solo desde un clic** en el indicador de conexión, nunca al cargar la página. Los
  navegadores bloquean o penalizan los pedidos que no vienen de una interacción.
- Sale solo con la pestaña oculta: con la pestaña a la vista ya está el banner.
- `tag = alert_code`: una subida de nivel reemplaza la notificación anterior de esa alerta en el sistema
  operativo.
- Al hacer clic, trae la pestaña al frente y abre el detalle de la alerta.
- El stream sigue abierto con la pestaña oculta. Si no, no llegarían estas notificaciones.

**Límite del navegador**: con HTTP/1.1, el navegador abre como máximo 6 conexiones por origen, y cada pestaña
ocupa una con el stream. Con muchas pestañas abiertas del Centro de Control, las demás peticiones pueden
quedar en espera.

## 7. Dónde está en el frontend

| Archivo | Qué hace |
|---|---|
| `src/services/realtime/alertStream.client.ts` | Cliente del stream: `EventSource`, backoff, reconexión con `?last_event_id=`, watchdog de 60 s, estados de conexión |
| `src/services/realtime/alertStream.types.ts` | Tipos de los eventos |
| `src/features/alerts/hooks/useAlertStream.ts` | Abre el stream una vez para toda la app (en `RootLayout`) y aplica los eventos a la caché de React Query |
| `src/features/alerts/lib/streamCache.ts` | Reglas de §4 como funciones puras, con pruebas |
| `src/features/alerts/hooks/useAlerts.ts` | `/active` y `/pending`: consultan cada 30 s solo si el stream no está en vivo |
| `src/features/alerts/lib/newAlerts.ts`, `hooks/useNewAlerts.ts` | Estado de los banners: deduplicación por `message_id`, agrupación por alerta y orden |
| `src/features/alerts/components/NewAlertToast.tsx` | Banner |
| `src/features/alerts/lib/alertSound.ts` | Sonido, desbloqueo del audio, una sola pestaña y silencio |
| `src/features/alerts/lib/browserNotification.ts` | Permiso y notificación del navegador |
| `src/features/alerts/components/RealtimeStatus.tsx` | Indicador de conexión y botones de sonido y notificaciones |

## 8. Probar en local

La guía completa está en [PRUEBA_LOCAL.md](PRUEBA_LOCAL.md): base de datos
local en Docker, backend y frontend. Un backend local con `NOTIFICATIONS_TEST_ENDPOINT=true` permite inventar
eventos sin escribir en la base:

```bash
curl -X POST localhost:8080/api/notifications/test-event -d '{"measuring_point_id":11556,"level":"AMARILLO"}'
# → alert_id -1: banner amarillo y sonido
curl -X POST localhost:8080/api/notifications/test-event -d '{"measuring_point_id":11556,"alert_id":-1,"level":"ROJO","code":"UP_ROJO"}'
# → el banner de la misma alerta pasa a rojo y titila
curl -X POST localhost:8080/api/notifications/test-event -d '{"measuring_point_id":11556,"alert_id":-1,"level":"ROJO","change":"CLASSIFIED","classification":"CONFIRMADA"}'
# → el banner deja de titilar
```

- Las alertas de prueba tienen ids negativos y código `ALR-PRUEBA-…`. Se ven en el banner, el sonido y la
  notificación del navegador, pero **no** en el mapa ni en el panel, porque no existen en la base.
- **Nunca** habilitar ese endpoint en el servidor del piloto: los eventos llegan a todos los navegadores
  conectados y, por push, a todos los celulares registrados.

La referencia completa de los endpoints está en [NOTIFICATIONS_API.MD](NOTIFICATIONS_API.MD).
