# API de Alert Management: pantalla de alertas

Todo lo que necesita el front para implementar la pantalla de alertas:
- listar las alertas con sus filtros;
- mostrar las columnas;
- ejecutar las acciones de cada fila: confirmar, falsa alerta, agregar etiqueta, ver detalle;
- exportar a Excel.

> **Evidencias fotográficas: quedan para después del MVP.** No hay endpoint para subir ni consultar fotos de una alerta (RN-09, HE-06, D-37 de la especificación). La gestión se hace con clasificación, etiqueta y observaciones de texto.

## Resumen de endpoints

| Para qué | Endpoint | `{id}` es… |
|---|---|---|
| Llenar el filtro "Dispositivo" | `GET /api/dashboard/devices` (módulo Dashboard) | — |
| Listar y filtrar | `GET /api/alerts` | — (el dispositivo va en `?device=`) |
| Exportar la búsqueda | `GET /api/alerts/export.xlsx` | — (el dispositivo va en `?device=`) |
| Ver detalle | `GET /api/alerts/{id}` | **ID de la ALERTA** |
| Confirmar / Falsa alerta | `POST /api/alerts/{id}/classification` | **ID de la ALERTA** |
| Agregar etiqueta | `POST /api/alerts/{id}/label` | **ID de la ALERTA** |
| Agregar observación (opcional, en el detalle) | `POST /api/alerts/{id}/observations` | **ID de la ALERTA** |

## Convenciones

- **Sin autenticación todavía (RN-10).** Desde la web, toda escritura lleva `"origin": "WEB"`.
- **`{id}` en `/api/alerts/{id}/...` es SIEMPRE el ID de la ALERTA, nunca el del dispositivo.** Se puede enviar el número (`id` de la fila, por ejemplo `123`) o el código (`code`, por ejemplo `ALR-000123`): son la misma alerta. El ID del dispositivo (`device.id`, el `measuring_point_id`, por ejemplo `11556`) **solo** se usa en el filtro `?device=` del listado y la exportación.

  | Correcto | Incorrecto |
  |---|---|
  | `POST /api/alerts/123/classification` | `POST /api/alerts/11556/classification` ← 11556 es el dispositivo |
  | `POST /api/alerts/ALR-000123/classification` | |

  Si por error se envía el ID de un dispositivo, el backend lo interpreta como el ID de una alerta. Lo más probable es `404 NOT_FOUND`, pero si existe una alerta con ese número, **se gestionaría otra alerta**. Tomar el ID siempre de `items[].id` o `items[].code` de la fila.
- **Fechas de respuesta**: hora de Colombia con offset, RFC 3339 (`2026-08-24T11:00:00-05:00`).
- **Fechas de filtro**: `dd/mm/aaaa` o `aaaa-mm-dd`, como días de Colombia.
- **Cuerpos JSON estrictos**: un campo desconocido o un JSON inválido ⇒ `400`.
- **Errores**: siempre `{ "code": "...", "message": "..." }`. El `message` está en español y se puede mostrar tal cual.

  | HTTP | `code` | Cuándo |
  |---|---|---|
  | 400 | `INVALID_INPUT` | Filtro, cuerpo o valor inválido |
  | 404 | `NOT_FOUND` | La alerta no existe |
  | 409 | `CONFLICT` | Etiquetar una alerta que todavía no está clasificada |
  | 500 | `CONSULTING_ALERTS`, `PERSISTING_ALERTS`, `CONSULTING_DEVICES` | Error de base de datos |

---

## 1. Filtro "Dispositivo": `GET /api/dashboard/devices`

Lo expone el módulo **Dashboard**, que tiene que estar habilitado en `AGATHA_MODULES`. `id` es el `measuring_point_id` que se envía en `device`.

```json
[
  { "id": 11556, "name": "PK34+500", "lastConnection": "2026-10-26T15:00:00Z", "lastBatteryLevel": 3.6 }
]
```

Agregar al principio la opción **"Todos"**: no enviar `device`, o enviar `device=todos`.

---

## 2. Listar y filtrar: `GET /api/alerts`

```
GET /api/alerts?device=&from=&to=&max_level=&event=&status=&page=
```

| Filtro de la pantalla | Parámetro | Valores | Regla |
|---|---|---|---|
| Dispositivo | `device` | `measuring_point_id`, o `todos` | Vacío = todos |
| Fecha inicio | `from` | `dd/mm/aaaa` o `aaaa-mm-dd` | Alertas que **iniciaron** desde ese día a las 00:00 |
| Fecha fin | `to` | `dd/mm/aaaa` o `aaaa-mm-dd` | Alertas que **iniciaron** hasta ese día a las 23:59 |
| Nivel de criticidad | `max_level` | `AMARILLO`, `NARANJA`, `ROJO` | Sobre el **nivel máximo** que alcanzó la alerta |
| Evento | `event` | `FUGA`, `MOVIMIENTO` | |
| Estado | `status` | `SIN_CLASIFICAR`, `CLASIFICADA_SIN_ETIQUETA`, `CLASIFICADA_CON_ETIQUETA` | |
| (paginación) | `page` | 1, 2, … | Default 1 |

- **Las dos fechas filtran por la fecha de INICIO de la alerta** (HU-9.1). "Fecha fin" no es la columna "Fin": es el último día de inicio a incluir.
- Con una sola fecha, el rango queda abierto. `to` anterior a `from` ⇒ `400` y no se busca: mostrar el `message`.
- **Multiselección** (nivel, evento, estado): repetir el parámetro (`max_level=NARANJA&max_level=ROJO`) o separar con comas (`max_level=NARANJA,ROJO`). No distingue mayúsculas.
- Entre filtros distintos se combinan con **Y**; dentro de un mismo filtro, con **O**.
- **10 filas por página**, de la más reciente a la más antigua por fecha de inicio. Páginas = `ceil(total / page_size)`.

Ejemplo: alertas naranja o rojo del dispositivo 11556, sin clasificar, que iniciaron en agosto.

```
GET /api/alerts?device=11556&from=01/08/2026&to=31/08/2026&max_level=NARANJA,ROJO&status=SIN_CLASIFICAR
```

Respuesta `200`:

```json
{
  "total": 23,
  "page": 1,
  "page_size": 10,
  "items": [
    {
      "id": 123,
      "code": "ALR-000123",
      "device": { "id": 11556, "pk": "PK34+500" },
      "event": "FUGA",
      "state": "ACTIVA",
      "started_at": "2026-08-24T11:00:00-05:00",
      "ended_at": null,
      "in_progress": true,
      "duration_hours": 6,
      "current_level": "ROJO",
      "max_level": "ROJO",
      "blinking": true,
      "classification": null,
      "management_status": "SIN_CLASIFICAR",
      "label": null,
      "label_other_text": null,
      "available_actions": ["CONFIRM", "FALSE_ALERT", "VIEW"],
      "leak_signature": { "d_pl": 4.6, "d_pl_min": 10, "confirmed": false }
    }
  ]
}
```

Sin resultados: `total: 0` e `items: []`.

### Columnas de la tabla

| Columna | Campo | Cómo mostrarlo |
|---|---|---|
| ID alerta | `code` | `ALR-000123` tal cual |
| Dispositivo ID | `device.id` | Número |
| Nombre dispositivo | `device.pk` | Es el nombre del dispositivo (`devices.name`). Si el dispositivo no está registrado, llega `MP-<id>` |
| Fin | `ended_at` / `in_progress` | `in_progress = true` ⇒ **"En curso"**. Si no, la fecha de `ended_at` |
| Nivel máximo | `max_level` | `AMARILLO` → Amarillo, `NARANJA` → Naranja, `ROJO` → Rojo |
| Evento | `event` | `FUGA` → Fuga, `MOVIMIENTO` → Movimiento |
| Clasificación | `classification` | `CONFIRMADA` → Confirmada, `FALSA_ALERTA` → Falsa alerta, `null` → vacío |
| Estado | `management_status` | `SIN_CLASIFICAR` → Sin clasificar, `CLASIFICADA_SIN_ETIQUETA` → Clasificada sin etiqueta, `CLASIFICADA_CON_ETIQUETA` → Clasificada con etiqueta |
| Etiqueta | `label` (+ `label_other_text`) | `null` → vacío. `OTRO` → mostrar `label_other_text`. El resto, según la tabla de etiquetas de la sección 5 |
| Acción | `available_actions` | Ver abajo |

Campos útiles que no son columnas: `started_at` (inicio), `current_level` (color actual, `null` si finalizó), `blinking` (`true` = ROJO sin clasificar, para resaltar la fila) y `duration_hours`.

**`leak_signature`** (en la fila y en el detalle) describe la **última hora calculada** de la alerta: `d_pl` es el % de registros de las últimas 24 h con planitud alta (`null` si la hora no lo trajo), `d_pl_min` el mínimo de la versión de umbrales y `confirmed` es `true` si la hora no se limitó y `d_pl >= d_pl_min`. Es `null` cuando el punto no tiene `d_pl_min` (la regla de planitud no está activa), no hay datos todavía o la alerta es anterior a este campo. Es solo informativo: no cambia el nivel ni las acciones.

**`duration_hours`** es la duración de la alerta en horas: el tiempo transcurrido desde `started_at` hasta el fin (o hasta la última hora procesada, si sigue en curso), **sin** las pausas entre un fin y una reactivación. Es el mismo número del mensaje de fin ("Duró N h") y lo que suman los colores. **Usar este campo; no calcular la duración en el front** (del primer color al último incluiría las pausas). No es `hours_calculated` del detalle: esas son las horas evaluadas, que cuentan también la hora de inicio y no cuentan las horas sin datos.

### Columna "Acción"

El backend dice qué botones mostrar en `available_actions`, según el estado:

| Estado | `available_actions` | Botones |
|---|---|---|
| Sin clasificar | `["CONFIRM", "FALSE_ALERT", "VIEW"]` | Confirmar · Falsa alerta · Ver detalle |
| Clasificada sin etiqueta | `["ADD_LABEL", "VIEW"]` | Agregar etiqueta · Ver detalle |
| Clasificada con etiqueta | `["VIEW"]` | Ver detalle |

`{id}` = `id` (o `code`) **de la fila**, es decir, de la alerta:

| Acción | Endpoint | Ejemplo con la fila `ALR-000123` |
|---|---|---|
| `CONFIRM` | `POST /api/alerts/{id}/classification` con `"classification": "CONFIRMADA"` | `POST /api/alerts/123/classification` |
| `FALSE_ALERT` | `POST /api/alerts/{id}/classification` con `"classification": "FALSA_ALERTA"` | `POST /api/alerts/123/classification` |
| `ADD_LABEL` | `POST /api/alerts/{id}/label` | `POST /api/alerts/123/label` |
| `VIEW` | `GET /api/alerts/{id}` | `GET /api/alerts/123` |

**Confirmar y Falsa alerta son el MISMO endpoint**: solo cambia el valor de `classification` en el cuerpo (ver sección 4).

Las acciones están disponibles tanto en alertas en curso como finalizadas. Clasificar o etiquetar **no** cierra la alerta.

---

## 3. Exportar: `GET /api/alerts/export.xlsx`

```
GET /api/alerts/export.xlsx?device=11556&from=01/08/2026&to=31/08/2026&max_level=NARANJA,ROJO&status=SIN_CLASIFICAR
```

- Recibe **los mismos filtros** que el listado y exporta **toda** la búsqueda, en el mismo orden. `page` se ignora.
- Responde el archivo `alertas_AAAAMMDD_HHMM.xlsx` con `Content-Disposition: attachment`. Desde el front: abrir la URL o descargarla como blob.
- **Más de 10.000 alertas ⇒ `400`**, sin archivo, con el mensaje "La búsqueda tiene más de 10.000 alertas. Ajuste los filtros para exportar."

Hojas del archivo:

- **Alertas**: ID de alerta, Dispositivo, PK, Inicio, Fin ("En curso"), Nivel actual ("—" si finalizó), Nivel máximo, Evento, Clasificación, Estado, Etiqueta, Observaciones, Origen y fecha-hora de la clasificación, Origen y fecha-hora de la etiqueta, Cambios de nivel.
- **Parámetros**: los filtros usados, la fecha de generación y el número de alertas.

---

## 4. Confirmar o Falsa alerta: `POST /api/alerts/{id}/classification`

`{id}` = **ID de la alerta** (`123` o `ALR-000123`), no del dispositivo.

### Por qué es un solo endpoint y por qué es POST

- **Un solo endpoint.** "Confirmada" y "Falsa alerta" son los dos valores posibles de una misma decisión: la **clasificación**. El botón "Confirmar" envía `CONFIRMADA` y el botón "Falsa alerta" envía `FALSA_ALERTA`, a la misma ruta. No existen `/confirm` ni `/false-alarm`.
- **POST, no PATCH ni PUT.** La clasificación no es un campo editable de la alerta: es un **registro que se crea una sola vez** (quién, desde dónde, cuándo y con qué observación) y **nunca se modifica**. La base de datos lo impide con un trigger.
  - `POST` = "crear la clasificación de esta alerta": la primera vez responde `201`; si ya existía, `409` y prevalece la primera.
  - `PATCH /api/alerts/{id}` sugeriría que la alerta se edita y que el valor se puede volver a cambiar. Ninguna de las dos cosas es cierta.
  - `PUT` promete reemplazar, y acá nunca se reemplaza.
- **Reintentos seguros.** Para que reintentar no duplique nada (la app sin conexión), se envía `client_record_id` (UUID). El mismo UUID otra vez ⇒ `200 ALREADY_RECEIVED`, sin efecto.

La etiqueta (sección 5) sigue la misma lógica: `POST /api/alerts/{id}/label` crea la etiqueta una sola vez.

### Cuerpo

`POST /api/alerts/123/classification`

```json
{
  "classification": "CONFIRMADA",
  "observation": "Se ve humedad en la abscisa",
  "origin": "WEB"
}
```

| Campo | Obligatorio | Regla |
|---|---|---|
| `classification` | Sí | `CONFIRMADA` o `FALSA_ALERTA` |
| `observation` | No | Hasta 500 caracteres |
| `origin` | Sí | `WEB` (o `APP_MOVIL` desde el celular) |
| `device_identifier` | Solo desde la app | Identificador del celular |
| `client_record_id` | No | UUID. Reenviar el mismo no duplica nada (reintentos de la app sin conexión) |
| `recorded_at` | No | Hora del registro en el celular sin conexión |

Respuestas:

| HTTP | `outcome` | Qué hacer en el front |
|---|---|---|
| `201` | `SAVED` | Reemplazar la fila con `alert`: el estado pasa a Clasificada sin etiqueta y la acción a Agregar etiqueta |
| `409` | `SAVED_AS_CONFLICT_OBSERVATION` | Otro usuario clasificó primero y **prevalece la primera**. Avisar al usuario con `existing` ("Ya estaba clasificada como Confirmada desde Web") y reemplazar la fila con `alert` |
| `200` | `ALREADY_RECEIVED` | El `client_record_id` ya se había recibido. Reemplazar la fila con `alert` |

```json
{
  "outcome": "SAVED",
  "alert": {
    "id": 123, "code": "ALR-000123", "device": { "id": 11556, "pk": "PK34+500" },
    "classification": "CONFIRMADA", "management_status": "CLASIFICADA_SIN_ETIQUETA",
    "available_actions": ["ADD_LABEL", "VIEW"]
  }
}
```

(`alert` trae siempre todos los campos de una fila del listado; acá se muestran solo algunos.)

```json
{
  "code": "CONFLICT",
  "outcome": "SAVED_AS_CONFLICT_OBSERVATION",
  "alert": { "...": "la fila con la clasificación que quedó" },
  "existing": { "classification": "CONFIRMADA", "classified_at": "2026-08-24T12:05:00-05:00", "origin": "WEB" }
}
```

---

## 5. Agregar etiqueta: `POST /api/alerts/{id}/label`

`{id}` = **ID de la alerta** (`123` o `ALR-000123`), no del dispositivo.

`POST /api/alerts/123/label`

```json
{
  "label": "OTRO",
  "other_text": "Rotura por raíz de árbol",
  "observation": "Reparado por la cuadrilla 1",
  "origin": "WEB"
}
```

El selector debe mostrar **solo las etiquetas de la clasificación** de la alerta (campo `classification` de la fila):

| Clasificación | `label` | Texto |
|---|---|---|
| `CONFIRMADA` | `FUGA_CONFIRMADA_CAMPO` | Fuga confirmada en campo |
| `CONFIRMADA` | `MOVIMIENTO_TIERRA` | Movimiento de tierra |
| `CONFIRMADA` | `MAQUINARIA_AJENA` | Intervención de maquinaria ajena |
| `CONFIRMADA` | `OTRO` | Otro ¿Cuál? |
| `FALSA_ALERTA` | `CLIMA_EXTREMO` | Condiciones climáticas extremas |
| `FALSA_ALERTA` | `TRANSITO_PESADO` | Tránsito de vehículo pesado |
| `FALSA_ALERTA` | `MANTENIMIENTO_PROGRAMADO` | Mantenimiento o intervención programada |
| `FALSA_ALERTA` | `OTRO` | Otro ¿Cuál? |

| Campo | Obligatorio | Regla |
|---|---|---|
| `label` | Sí | De la lista de su clasificación. Si no ⇒ `400` |
| `other_text` | Solo con `OTRO` | 1 a 100 caracteres. Con otra etiqueta **no se envía** (si se envía ⇒ `400`) |
| `observation` | No | Hasta 500 caracteres |
| `origin`, `device_identifier`, `client_record_id`, `recorded_at` | | Igual que en la clasificación |

Respuestas: las mismas que la clasificación (`201` / `409` / `200`). Además, si la alerta todavía no está clasificada ⇒ `409` con `{ "code": "CONFLICT", "message": "la alerta todavía no está clasificada: primero clasificar, después etiquetar" }`. Con `201`, el estado pasa a Clasificada con etiqueta y la acción a Ver detalle.

---

## 6. Ver detalle: `GET /api/alerts/{id}`

`{id}` = **ID de la alerta** (`123` o `ALR-000123`), no del dispositivo.

```
GET /api/alerts/123
GET /api/alerts/ALR-000123      (misma alerta)
```

Devuelve todos los campos de la fila, más:

| Campo | Descripción |
|---|---|
| `opened_at`, `last_evaluated_at` | Apertura y última hora procesada |
| `reactivation_count`, `last_reactivated_at` | Veces que se reactivó (volvió dentro de 24 h sin clasificar) |
| `opening_indicator_value`, `last_indicator_value`, `max_indicator_value` | Indicador al abrir, en la última hora y el máximo de la alerta |
| `hours_calculated`, `hours_no_data` | Horas con y sin cálculo |
| `low_confidence_at_open` | Abrió con confianza baja (dato preliminar) |
| `classification_detail` | `{ value, at, origin, device_identifier, recorded_at }` o `null` |
| `label_detail` | `{ value, other_text, at, origin, device_identifier, recorded_at }` o `null` |
| `observations[]` | `{ id, text, kind, step, origin, device_identifier, recorded_at, received_at }`. `kind`: `OBSERVACION`, `CONFLICTO_CLASIFICACION` o `CONFLICTO_ETIQUETA` |
| `levels[]` | Línea de tiempo por color, en orden: `{ seq, opened_by, level, started_at, ended_at, messages[] }`. `opened_by`: `OPEN`, `UP`, `DOWN` o `REACTIVATION` |
| `levels[].messages[]` | `{ id, at, kind, code, level, indicator_value, title, body }`. `title` y `body` ya vienen redactados para mostrar |

Desde el detalle se pueden ejecutar las mismas acciones de la fila, según `available_actions`, y agregar observaciones libres:

```
POST /api/alerts/123/observations        ({id} = ID de la alerta)
{ "text": "Se programó visita para mañana", "origin": "WEB" }
```

`text` tiene de 1 a 500 caracteres. Las observaciones solo se agregan: no se editan ni se borran (RN-08). La respuesta es `201` con `outcome: SAVED`.

---

### Evento `alert_management.alert_changed`

El mismo objeto viaja en la foto de la alerta que se publica con cada cambio visible, como `leak_signature` (misma forma o `null`), junto a `current_level`, `max_level`, `blinking`, `management_status`, etc. La foto se publica solo cuando cambia algo visible (nivel, estado, horas sin datos o gestión): un cambio de `confirmed` sin cambio de nivel no publica por sí mismo; el valor vigente siempre está en `GET /api/alerts/{id}`.

## Fuera de alcance

- **Evidencias fotográficas**: después del MVP.
- **Notificaciones (web y push)**: no son de este módulo. Alert Management publica `alert_management.alert_notification` y la entrega es del módulo de notificaciones.
- **Umbrales**: se configuran en el Rule Engine (`/api/rule-engine/...`).
- **Otros endpoints del módulo** que esta pantalla no necesita: `GET /api/alerts/pending` (panel "Sin clasificar"), `GET /api/alerts/active` (mapa), `GET /api/alerts/devices/{id}/alerts` (historial de un dispositivo; **es la única ruta donde `{id}` es el ID del DISPOSITIVO**, por el prefijo `/devices/`) y `GET|POST /api/alerts/policies` (configuración del analista). Están descritos en `ALERT_MANAGEMENT_README.MD`.
