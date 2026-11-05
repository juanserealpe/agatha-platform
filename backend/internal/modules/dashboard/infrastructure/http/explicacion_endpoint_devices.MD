# Endpoint de dispositivos del dashboard

Devuelve la lista de dispositivos registrados, cada uno con su última conexión y su último nivel de batería. Es el punto de partida de la pantalla: de acá sale el `id` que piden los demás endpoints como `deviceId`.

- URL base: `https://129.213.118.230` (HTTPS, sin puerto)
- Método: `GET`
- Autenticación: ninguna
- Solo lectura

## Ruta

```
https://129.213.118.230/api/dashboard/devices
```

No recibe parámetros.

## Respuesta

`200 OK`

```json
[
  {
    "id": 11556,
    "name": "Sensor 11556",
    "lastConnection": "2026-10-13T16:45:41Z",
    "lastBatteryLevel": 87.5
  },
  {
    "id": 11557,
    "name": "Sensor 11557",
    "lastConnection": null,
    "lastBatteryLevel": null
  }
]
```

| Campo | Tipo | Qué es |
|---|---|---|
| `id` | entero | Identificador del dispositivo (punto de medición). Es el `deviceId` de `/measurements` y `/series` |
| `name` | texto | Nombre del dispositivo |
| `lastConnection` | fecha RFC3339 o `null` | Fecha y hora del dato más reciente que se recibió del dispositivo |
| `lastBatteryLevel` | número o `null` | Último nivel de batería registrado, en porcentaje |

## Qué tener en cuenta

- **Devuelve todos los dispositivos registrados**, estén activos o no. La respuesta no trae un campo que los distinga.
- **Sin dispositivos devuelve una lista vacía** (`[]`), no un error.
- **`lastConnection` y `lastBatteryLevel` pueden ser `null`.** Pasa con un dispositivo que todavía no envió datos, o que no reporta batería. La pantalla debe contemplarlo.
- **El orden de la lista no está garantizado.** Si se necesita un orden fijo (por nombre o por id), hay que ordenar en el frontend.
- **`lastConnection` está en UTC.** Para mostrar hora de Colombia hay que restar 5 horas.
- **`lastConnection` es la hora del último dato, no un estado en línea.** Para decidir si un dispositivo está "conectado" hay que comparar esa fecha con la hora actual; los dispositivos envían una lectura cada 15 minutos aproximadamente.

## Errores

```json
{ "code": "CONSULTING_DEVICES", "message": "no fue posible consultar los dispositivos" }
```

| HTTP | `code` | Cuándo |
|---|---|---|
| 500 | `CONSULTING_DEVICES` | Falló la consulta a la base de datos |

## Endpoints relacionados

| Endpoint | Para qué | Documento |
|---|---|---|
| `GET /api/dashboard/series` | Series de las variables que se grafican, por sección | `explicacion_endpoint_series.MD` |
| `GET /api/dashboard/measurements` | Todas las mediciones crudas de un dispositivo | `explicacion_endpoint_measurements.MD` |
