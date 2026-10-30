# Endpoint de mediciones del dashboard

Devuelve **todas** las mediciones crudas que envió un dispositivo en un rango de fechas, agrupadas por métrica. No permite elegir métricas: trae todas las que tengan datos en el rango.

- URL base: `https://129.213.118.230` (HTTPS, sin puerto)
- Método: `GET`
- Autenticación: ninguna
- Solo lectura

## Cuándo usar este y cuándo `/series`

| Necesidad | Endpoint |
|---|---|
| Graficar variables puntuales (temperatura, B1, B2, deriva…) | `/api/dashboard/series` |
| Graficar el indicador de fuga o la densidad | `/api/dashboard/series` (acá no están) |
| Ver todo lo que envió el sensor, sin elegir | `/api/dashboard/measurements` |
| Una métrica que todavía no está en el catálogo de `/series` (batería, voltaje, RMS…) | `/api/dashboard/measurements` |

Este endpoint solo trae lecturas del sensor. El indicador de fuga y la densidad los calcula el sistema y solo salen por `/series`.

## Ruta

```
https://129.213.118.230/api/dashboard/measurements?deviceId=11556&startDate=2026-07-28T00:00:00Z&endDate=2026-07-29T00:00:00Z
```

## Parámetros

| Parámetro | Obligatorio | Formato | Ejemplo |
|---|---|---|---|
| `deviceId` | Sí | Entero positivo | `11556` |
| `startDate` | Sí | RFC3339 | `2026-07-28T00:00:00Z` |
| `endDate` | Sí | RFC3339 | `2026-07-29T00:00:00Z` |

- El rango es `[startDate, endDate)`: incluye `startDate` y excluye `endDate`.
- `startDate` debe ser anterior a `endDate`, y ninguna de las dos puede ser futura.
- El `deviceId` es el `id` que devuelve `/api/dashboard/devices`.

## Respuesta

`200 OK`

```json
{
  "deviceId": 11556,
  "startDate": "2026-07-28T00:00:00Z",
  "endDate": "2026-07-29T00:00:00Z",
  "measurements": [
    {
      "metricId": 20,
      "metricName": "Temperatura (°C)",
      "unit": "°",
      "data": [
        { "timestamp": "2026-07-28T00:00:00Z", "value": 18.4 },
        { "timestamp": "2026-07-28T00:15:00Z", "value": 18.2 }
      ]
    },
    {
      "metricId": 13,
      "metricName": "B2 fuga 1200-2000Hz",
      "unit": "-",
      "data": [
        { "timestamp": "2026-07-28T00:00:00Z", "value": 54 },
        { "timestamp": "2026-07-28T00:15:00Z", "value": 55 }
      ]
    }
  ]
}
```

| Campo | Qué es |
|---|---|
| `deviceId`, `startDate`, `endDate` | Lo mismo que se pidió |
| `measurements` | Una entrada por métrica con datos en el rango |
| `metricId` | Identificador de la métrica (ver catálogo abajo) |
| `metricName`, `unit` | Nombre y unidad para mostrar |
| `data` | Las lecturas de esa métrica, ordenadas por tiempo |

## Qué tener en cuenta

- **El orden de las métricas no está garantizado.** Puede cambiar entre una llamada y otra. Hay que buscar cada métrica por su `metricId`, nunca por su posición en la lista.
- **Una métrica sin datos en el rango no aparece.** No viene con `data` vacío: directamente falta. Sin ninguna lectura, `measurements` es `[]`.
- **Es una respuesta pesada.** Un dispositivo envía hasta 21 métricas cada 15 minutos aproximadamente: unas 2.000 lecturas por día. Conviene pedir rangos cortos; para rangos largos o gráficas, usar `/series` con las variables necesarias.
- **Las horas están en UTC.** Para mostrar hora de Colombia hay que restar 5 horas. Para pedir un día de Colombia se puede enviar el offset: `2026-07-28T00:00:00-05:00`.
- **Los huecos son reales.** Si el dispositivo no envió datos en un periodo, no hay lecturas; no se rellena con cero.

## Catálogo de métricas

Los nombres y las unidades de la respuesta salen de la base de datos; esta tabla es la referencia de qué es cada `metricId`.

| `metricId` | Métrica | Unidad | Categoría |
|---|---|---|---|
| 1 | Pitch | ° | Orientación |
| 2 | Roll | ° | Orientación |
| 3 | Voltaje | V | Energía |
| 4 | Corriente | mA | Energía |
| 5 | Batería | % | Energía |
| 6 | RMS | ADC | Vibración |
| 7 | Actividad | — | Vibración |
| 8 | Magnitud promedio | g | Acelerómetro |
| 9 | Actividad pico | g/s | Acelerómetro |
| 10 | Razón R (fuga/viento) | — | Acústica |
| 11 | B0 viento 30-300 Hz | — | Espectral |
| 12 | B1 fuga 500-1200 Hz | — | Espectral |
| 13 | B2 fuga 1200-2000 Hz | — | Espectral |
| 14 | B3 guarda 2-2,4 kHz | — | Espectral |
| 15 | Pitch base | ° | Orientación |
| 16 | Roll base | ° | Orientación |
| 17 | RMS acústico | ADC | Acústica |
| 18 | Planitud espectral | — | Espectral |
| 19 | Deriva | ° | Orientación |
| 20 | Temperatura | °C | Temperatura |
| 21 | Humedad | % | Humedad |

## Errores

Todos tienen la misma forma:

```json
{ "code": "INVALID_INPUT", "message": "debe indicar una fecha de inicio" }
```

| HTTP | `code` | Cuándo |
|---|---|---|
| 400 | `INVALID_INPUT` | Falta `deviceId`, `startDate` o `endDate`; `deviceId` no es un entero positivo; una fecha no es RFC3339; o `startDate` no es anterior a `endDate` |
| 400 | `FUTURE_DATE` | `startDate` o `endDate` es futura |
| 404 | `NOT_FOUND` | El dispositivo no existe |
| 500 | `CONSULTING_MEASUREMENTS` | Falló la consulta a la base de datos |

## Endpoints relacionados

| Endpoint | Para qué | Documento |
|---|---|---|
| `GET /api/dashboard/devices` | Lista de dispositivos y sus `id` | `explicacion_endpoint_devices.MD` |
| `GET /api/dashboard/series` | Series de las variables que se grafican, por sección | `explicacion_endpoint_series.MD` |
