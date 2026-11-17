# Planitud espectral en el dashboard

La planitud espectral ya se puede graficar. Se pide igual que cualquier otra variable del endpoint de series, con la clave `planitud`.

- URL base: `https://129.213.118.230` (HTTPS, sin puerto)
- Método: `GET`
- Autenticación: ninguna
- Solo lectura

## Qué es

Es una lectura del sensor, como B1 y B2: el dispositivo la envía en cada registro. Mide qué tan "plano" es el espectro del ruido. El sistema la usa para reconocer la firma de una fuga.

| Clave | Nombre | Unidad | `source` | Frecuencia |
|---|---|---|---|---|
| `planitud` | Planitud espectral | — (sin unidad) | `measurement` | Una por lectura (~15 min) |

Pertenece a la sección **Fuga**, junto a `b1`, `b2`, `indicador` y `densidad`.

## Cómo pedirla

Sola:

```
GET https://129.213.118.230/api/dashboard/series?deviceId=11556&startDate=2026-07-28T00:00:00Z&endDate=2026-07-31T00:00:00Z&variables=planitud
```

Con el resto de la sección Fuga:

```
GET https://129.213.118.230/api/dashboard/series?deviceId=11556&startDate=2026-07-28T00:00:00Z&endDate=2026-07-31T00:00:00Z&variables=b1,b2,planitud,indicador,densidad
```

| Parámetro | Obligatorio | Formato | Ejemplo |
|---|---|---|---|
| `deviceId` | Sí | Entero positivo | `11556` |
| `startDate` | Sí | RFC3339 | `2026-07-28T00:00:00Z` |
| `endDate` | Sí | RFC3339 | `2026-07-31T00:00:00Z` |
| `variables` | No | Claves separadas por coma | `planitud` |

- El rango es `[startDate, endDate)`: incluye `startDate` y excluye `endDate`.
- Ninguna fecha puede ser futura, y `startDate` debe ser anterior a `endDate`.

Ejemplo con `fetch`:

```ts
const params = new URLSearchParams({
  deviceId: "11556",
  startDate: "2026-07-28T00:00:00Z",
  endDate: "2026-07-31T00:00:00Z",
  variables: "planitud",
});

const res = await fetch(`https://129.213.118.230/api/dashboard/series?${params}`);
if (!res.ok) {
  const error = await res.json(); // { code, message }
  throw new Error(`${error.code}: ${error.message}`);
}

const body = await res.json();
const planitud = body.series.find((s) => s.key === "planitud");
// planitud.data: [{ timestamp, value }, ...]
```

## Respuesta

`200 OK`

```json
{
  "deviceId": 11556,
  "startDate": "2026-07-28T00:00:00Z",
  "endDate": "2026-07-31T00:00:00Z",
  "series": [
    {
      "key": "planitud",
      "name": "Planitud espectral",
      "unit": "-",
      "source": "measurement",
      "data": [
        { "timestamp": "2026-07-28T00:00:00Z", "value": 0.42 },
        { "timestamp": "2026-07-28T00:15:00Z", "value": 0.44 }
      ]
    }
  ]
}
```

Los valores del ejemplo son ilustrativos. Busquen la serie por `key`, no por posición: el arreglo `series` viene siempre en el orden del catálogo (`b1`, `b2`, `planitud`, `indicador`, `densidad`, ...), sin importar el orden en que se pidieron.

La variable también aparece en `GET /api/dashboard/variables`:

```json
{ "key": "planitud", "name": "Planitud espectral", "unit": "-", "source": "measurement" }
```

## Qué tener en cuenta al graficar

- **Las horas están en UTC.** Para hora de Colombia hay que restar 5 horas, o pedir con offset: `2026-07-28T00:00:00-05:00`.
- **Misma frecuencia que B1 y B2** (un punto por lectura), así que puede compartir el eje X con ellas sin remuestrear.
- **No tiene la escala de B1 y B2.** Si va en la misma gráfica necesita su propio eje Y. Lo más claro es una gráfica aparte.
- **No fijen el eje Y a mano.** El backend no define un rango fijo para esta variable: escalen el eje con los datos.
- **Los huecos son reales.** Si el dispositivo no envió la planitud en un registro, no hay punto. No lo rellenen con cero.
- **`data` puede venir vacío (`[]`)** si no hay lecturas en el rango. Muestren "sin datos", no un error.

## No confundir con la "firma de fuga"

En los mensajes de alerta aparece la **firma de fuga** (`d_pl`): el porcentaje de registros de las últimas 24 h con planitud alta. Es otra cosa:

| | `planitud` (este endpoint) | Firma de fuga (`d_pl`) |
|---|---|---|
| Qué es | La lectura cruda del sensor | Un porcentaje calculado a partir de la planitud |
| Unidad | Sin unidad | % (de 0 a 100) |
| Frecuencia | Una por lectura (~15 min) | Una por hora |
| Dónde está | `/api/dashboard/series?variables=planitud` | En la alerta (`leak_signature`), no en el dashboard |

## Errores

Todos tienen la forma `{ "code": "...", "message": "..." }`.

| HTTP | `code` | Cuándo |
|---|---|---|
| 400 | `INVALID_INPUT` | Falta un parámetro obligatorio, una fecha no es RFC3339, `startDate` no es anterior a `endDate`, o `variables` trae una clave que no existe |
| 400 | `FUTURE_DATE` | `startDate` o `endDate` es futura |
| 404 | `NOT_FOUND` | El dispositivo no existe |
| 500 | `CONSULTING_MEASUREMENTS` | Falló la consulta a la base de datos |

El resto del endpoint (todas las variables y secciones) está en `explicacion_endpoint_series.MD`.
