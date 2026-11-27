# Guía de integración móvil: API key

Para: el equipo de la app móvil.
Complementa `backend/internal/modules/alert_management/MOBILE_ALERTS_API_GUIDE.md` (endpoints, modelos y errores de alertas).

Esta guía describe **qué espera recibir el backend**. Cómo se implementa dentro de la app queda a criterio del equipo móvil.

## 1. Qué cambia

Una sola cosa: **toda** petición de la app al backend tiene que llevar este encabezado HTTP:

```http
X-API-Key: <key de la app>
```

Nada más:

- No hay login ni pantalla de usuario y contraseña.
- No hay token que venza ni sesión que se cierre. La app puede quedar abierta o en segundo plano por meses y sigue funcionando.
- El body, los parámetros y las respuestas de los endpoints no cambian.
- Las notificaciones push (FCM) no cambian: no pasan por este control.

## 2. Qué key usar

| Entorno | Key | Quién la entrega |
|---|---|---|
| Producción (VPS) | Key de producción | El equipo de backend, por un canal privado (nunca por chat abierto, correo masivo ni repositorio) |
| Pruebas / local | Key de pruebas, distinta de la de producción | El equipo de backend |

- La key es un texto de 43 caracteres (letras, números, `-` y `_`), por ejemplo: `q3Vt9xK2...` (ilustrativo, no es una key real).
- Se envía **tal cual**, sin prefijos (no `Bearer`, no comillas) y sin espacios.
- Es la **misma** key para todos los celulares de una misma build. No es por usuario ni por dispositivo.
- La app **no** la genera, no la pide al backend y no la cambia en tiempo de ejecución: viene fija en la build.

### Cómo manejarla

- **No** escribirla dentro del código fuente que se sube al repositorio. Inyectarla al compilar (configuración de build, variables del CI o un archivo local no versionado), según lo que use la app.
- Agregar el encabezado en **un único punto** del cliente HTTP (el lugar por donde salen todas las peticiones), no en cada llamada por separado. Así no se olvida en ningún endpoint y cambiarla es tocar un solo lugar.
- **No** escribirla en logs, reportes de errores ni herramientas de analítica (cuidado con los logs que vuelcan encabezados completos).
- Usar siempre **HTTPS**. Por HTTP la key viaja en texto plano.

## 3. A qué puede acceder la key

| Uso | Endpoint |
|---|---|
| Alertas activas | `GET /api/alerts/active` |
| Pendientes de clasificar | `GET /api/alerts/pending` |
| Historial con filtros | `GET /api/alerts` |
| Detalle de una alerta | `GET /api/alerts/{id}` |
| Historial de un sensor | `GET /api/alerts/devices/{id}/alerts` |
| Clasificar | `POST /api/alerts/{id}/classification` |
| Etiquetar | `POST /api/alerts/{id}/label` |
| Agregar observación | `POST /api/alerts/{id}/observations` |
| Registrar token FCM | `PUT /api/devices/push-token` |
| Dashboard (solo lectura) | `GET /api/dashboard/devices`, `/measurements`, `/variables`, `/series` |
| Comprobar la key | `GET /api/auth/me` |

Cualquier otra ruta (umbrales, políticas, exportación a Excel, stream en tiempo real de la web) responde `403` aunque la key sea correcta. Es a propósito: la key queda dentro de la app instalada y se puede extraer, y este alcance limita lo que se puede hacer con ella si se filtra.

Si la app necesita un endpoint que no está en la tabla, pedirlo al equipo de backend. No es algo que se resuelva del lado de la app.

## 4. Respuestas nuevas

Formato igual al resto de la API: `{"code": "<CODIGO>", "message": "<texto en español>"}`.

| Status | `code` | Significado | Qué se espera de la app |
|---|---|---|---|
| `401` | `UNAUTHORIZED` | Falta el encabezado o la key es incorrecta (por ejemplo, una versión vieja de la app después de cambiar la key) | No reintentar: no se arregla solo. Mostrar un aviso del tipo "Actualice la aplicación para seguir recibiendo datos" |
| `403` | `FORBIDDEN` | La key es correcta pero la ruta no está permitida para la app | Es un error de integración: registrarlo y avisar a backend. No reintentar |

Ninguno de los dos debe entrar a la cola de reintentos sin conexión (`MOBILE_ALERTS_API_GUIDE.md` §7.6): son errores definitivos, como el resto de los `4xx`.

## 5. Comprobar la key (recomendado al abrir la app)

```http
GET /api/auth/me
X-API-Key: <key>
```

| Respuesta | Significado |
|---|---|
| `200 {"principal": "mobile"}` | La key es válida |
| `401` | La key de esta versión ya no sirve: mostrar el aviso de actualizar |

## 6. Cambio de key (rotación)

Si hay que cambiar la key (por ejemplo, porque se filtró), el proceso es:

1. Backend acepta la key vieja y la nueva **al mismo tiempo**.
2. Se publica una versión de la app con la key nueva.
3. Cuando ya nadie usa la versión vieja, backend deja solo la nueva. Desde ahí, la versión vieja recibe `401` (por eso el aviso del §4 y §5).

La app no hace nada especial: solo compila con la key que le entregue backend.

## 7. Fechas: cuándo pasa a ser obligatoria

Hoy el backend **acepta** peticiones sin key y solo las registra, para que la versión ya instalada siga funcionando. Cuando la versión nueva (con la key) esté instalada en los celulares, backend activa el modo obligatorio y desde ese momento **toda petición sin key correcta recibe `401`**. Esa fecha se coordina entre los dos equipos: avisar a backend cuando la versión nueva esté publicada.

## 8. Qué es y qué no es esta key

- **Es** un control de acceso básico: frena el tráfico anónimo, se puede revocar y limita qué rutas se tocan.
- **No es** un secreto fuerte: cualquiera con el instalador de la app puede extraerla. No guardar nada más sensible al lado ni basar otra seguridad en ella.

## 9. Checklist

- [ ] Key recibida de backend por canal privado (una para pruebas y otra para producción).
- [ ] Key inyectada al compilar, fuera del repositorio.
- [ ] Encabezado `X-API-Key` agregado en un único punto, presente en **todas** las peticiones.
- [ ] `401` y `403` sin reintento y fuera de la cola sin conexión.
- [ ] Aviso de "actualice la aplicación" ante `401`.
- [ ] (Recomendado) comprobación con `GET /api/auth/me` al abrir la app.
- [ ] Key fuera de logs y reportes de errores.
- [ ] Probado contra un backend en modo obligatorio antes de publicar.

## 10. Prueba rápida (sin la app)

Para confirmar que la key funciona antes de integrarla:

```bash
curl -i https://<host>/api/auth/me -H "X-API-Key: <key>"
# 200 {"principal":"mobile"}

curl -i https://<host>/api/alerts/active -H "X-API-Key: <key>"
# 200 con las alertas

curl -i https://<host>/api/alerts/active
# 401 (sin key), una vez activado el modo obligatorio
```
