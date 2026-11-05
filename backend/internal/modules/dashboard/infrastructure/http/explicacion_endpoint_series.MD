# Endpoint de series del dashboard

Devuelve, para un dispositivo y un rango de fechas, las series de tiempo de las variables que se pidan. Todas las variables salen con la misma forma (`timestamp`, `value`), así que se grafican con el mismo componente.

- URL base: `https://129.213.118.230` (HTTPS, sin puerto)
- Método: `GET`
- Autenticación: ninguna
- Solo lectura

## Rutas

| Ruta | Qué devuelve |
|---|---|
| `GET /api/dashboard/series` | Las series de las variables pedidas |
| `GET /api/dashboard/variables` | La lista de variables disponibles, sin datos |

## Parámetros de `/api/dashboard/series`

| Parámetro | Obligatorio | Formato | Ejemplo |
|---|---|---|---|
| `deviceId` | Sí | Entero positivo | `11556` |
| `startDate` | Sí | RFC3339 | `2026-07-28T00:00:00Z` |
| `endDate` | Sí | RFC3339 | `2026-07-31T00:00:00Z` |
| `variables` | No | Claves separadas por coma | `b1,b2,indicador` |

- El rango es `[startDate, endDate)`: incluye `startDate` y excluye `endDate`.
- `startDate` debe ser anterior a `endDate`, y ninguna de las dos puede ser futura.
- Sin `variables` devuelve **todas** las del catálogo. Conviene pedir solo las que se van a graficar.
- Las claves no distinguen mayúsculas y se ignoran los espacios.

## Variables por sección

El sistema muestra las gráficas en cuatro secciones. La sección es una agrupación de la pantalla: el endpoint no la devuelve, se arma pidiendo las claves de cada una.

### Ambiente

| Clave | Variable | Unidad | Frecuencia |
|---|---|---|---|
| `temperatura` | Temperatura | °C | Una por lectura (~15 min) |
| `humedad` | Humedad | % | Una por lectura (~15 min) |

```
https://129.213.118.230/api/dashboard/series?deviceId=11556&startDate=2026-07-28T00:00:00Z&endDate=2026-07-31T00:00:00Z&variables=temperatura,humedad
```

### Fuga

| Clave | Variable | Unidad | Frecuencia |
|---|---|---|---|
| `b1` | B1 fuga 500-1200 Hz | — | Una por lectura (~15 min) |
| `b2` | B2 fuga 1200-2000 Hz | — | Una por lectura (~15 min) |
| `indicador` | Indicador de fuga I, ventana de 24 h | — (de 0 a 1) | Una por hora |
| `densidad` | Densidad D, ventana de 24 h | % (de 0 a 100) | Una por hora |

```
https://129.213.118.230/api/dashboard/series?deviceId=11556&startDate=2026-07-28T00:00:00Z&endDate=2026-07-31T00:00:00Z&variables=b1,b2,indicador,densidad
```

**Umbral del indicador de fuga: pendiente.** La gráfica del indicador debe llevar una línea de umbral, pero su valor **está por validarse** y el endpoint todavía no lo devuelve. Hasta que se defina:

- No hay un valor oficial que dibujar.
- Cuando se valide, estará en la misma escala del indicador (de 0 a 1).
- Este documento se actualizará con el valor y con la forma en que llega.

### Movimiento

| Clave | Variable | Unidad | Frecuencia |
|---|---|---|---|
| `deriva` | Deriva | ° | Una por lectura (~15 min) |
| `pitch` | Pitch | ° | Una por lectura (~15 min) |
| `roll` | Roll | ° | Una por lectura (~15 min) |
| `aceleracion` | Aceleración (magnitud promedio) | g | Una por lectura (~15 min) |

`pitch` y `roll` son dos series separadas; se pueden dibujar en la misma gráfica porque comparten unidad.

```
https://129.213.118.230/api/dashboard/series?deviceId=11556&startDate=2026-07-28T00:00:00Z&endDate=2026-07-31T00:00:00Z&variables=deriva,pitch,roll,aceleracion
```

### Energía

| Clave | Variable | Unidad | Frecuencia |
|---|---|---|---|
| `voltaje` | Voltaje | V | Una por lectura (~15 min) |
| `corriente` | Corriente | mA | Una por lectura (~15 min) |

Tienen unidades distintas: si van en la misma gráfica, cada una necesita su propio eje Y.

```
https://129.213.118.230/api/dashboard/series?deviceId=11556&startDate=2026-07-28T00:00:00Z&endDate=2026-07-31T00:00:00Z&variables=voltaje,corriente
```

## Respuesta de `/api/dashboard/series`

`200 OK`

```json
{
  "deviceId": 11556,
  "startDate": "2026-07-28T00:00:00Z",
  "endDate": "2026-07-31T00:00:00Z",
  "series": [
    {
      "key": "b2",
      "name": "B2 fuga 1200-2000Hz",
      "unit": "-",
      "source": "measurement",
      "data": [
        { "timestamp": "2026-07-28T00:00:00Z", "value": 54 },
        { "timestamp": "2026-07-28T00:15:00Z", "value": 55 }
      ]
    },
    {
      "key": "indicador",
      "name": "Indicador I (24 h)",
      "unit": "-",
      "source": "evaluation",
      "data": [
        { "timestamp": "2026-07-28T00:00:00Z", "value": 0.9942 },
        { "timestamp": "2026-07-28T01:00:00Z", "value": 0.9939 }
      ]
    }
  ]
}
```

| Campo | Qué es |
|---|---|
| `key` | La clave con la que se pidió la variable |
| `name`, `unit` | Nombre y unidad para mostrar |
| `source` | `measurement` (lectura del sensor) o `evaluation` (calculada por el sistema cada hora) |
| `data` | Los puntos, ordenados por tiempo. Vacío (`[]`) si no hay datos en el rango |

Las series vienen siempre en el mismo orden (el del catálogo), sin importar el orden en que se pidieron.

## Respuesta de `/api/dashboard/variables`

`200 OK`

```json
[
  { "key": "b1", "name": "B1 fuga 500-1200Hz", "unit": "-", "source": "measurement" },
  { "key": "indicador", "name": "Indicador I (24 h)", "unit": "-", "source": "evaluation" }
]
```

Sirve para no dejar escritas a mano las claves, los nombres y las unidades en el frontend.

## Qué tener en cuenta al graficar

- **Las horas están en UTC.** Para mostrar hora de Colombia hay que restar 5 horas. Para pedir un día de Colombia se puede enviar el offset: `2026-07-28T00:00:00-05:00`.
- **Dos frecuencias distintas.** Las variables de `source: measurement` traen un punto por lectura; `indicador` y `densidad` traen uno por hora.
- **El punto del indicador y de la densidad resume las 24 horas anteriores.** El `timestamp` es el final de esa ventana.
- **Los huecos son reales.** Si una hora no se pudo calcular (por ejemplo, por falta de datos), no hay punto. No se rellena con cero.
- **Escalas distintas en la sección Fuga.** `indicador` va de 0 a 1 y `densidad` de 0 a 100: necesitan ejes o gráficas separadas.
- **`indicador` y `densidad` pueden venir vacías** si el sistema todavía no calculó ese rango.

## Errores

Todos tienen la misma forma:

```json
{ "code": "INVALID_INPUT", "message": "alguna de las variables solicitadas no existe" }
```

| HTTP | `code` | Cuándo |
|---|---|---|
| 400 | `INVALID_INPUT` | Falta `deviceId`, `startDate` o `endDate`; `deviceId` no es un entero positivo; una fecha no es RFC3339; `startDate` no es anterior a `endDate`; o `variables` trae una clave que no existe |
| 400 | `FUTURE_DATE` | `startDate` o `endDate` es futura |
| 404 | `NOT_FOUND` | El dispositivo no existe |
| 500 | `CONSULTING_MEASUREMENTS` | Falló la consulta a la base de datos |

## Agregar una variable

Las variables viven en un catálogo: `internal/modules/dashboard/domain/entities/series_catalog.go`. Agregar una es sumar una línea ahí; el endpoint, el servicio y las consultas no cambian. Una variable nueva aparece sola en `/api/dashboard/variables`.

Después de agregarla, hay que sumarla a la sección que corresponda en este documento.
