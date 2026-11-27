# Módulo: Auth

**Responsabilidad**: autenticar las peticiones HTTP de la plataforma con dos credenciales:

| Cliente | Credencial | Cómo se envía | Alcance |
|---|---|---|---|
| Web (admin) | JWT HS256 emitido por `POST /api/auth/login` | `Authorization: Bearer <token>`; solo en el stream SSE, `?access_token=<token>` | Toda la API |
| App móvil | API key fija de la build | `X-API-Key: <key>` | Solo las rutas de `mobileRoutes` (ver abajo) |
| Dataflow Labs | `AGATHA_CALLBACK_TOKEN` (propio del callback) | `Authorization: Bearer` | Solo `/ingestion/callback` (ruta pública para el middleware) |

El servidor **no deduce** el tipo de cliente por User-Agent, Origin ni ningún otro encabezado (se falsifican). La credencial identifica al cliente: `X-API-Key` => móvil; Bearer => admin.

Hay un solo usuario web (admin), configurado por entorno. No hay tabla de usuarios ni logout en el servidor: cambiar `AGATHA_JWT_SECRET` invalida todas las sesiones.

## Estructura

```
auth/
  application/ports      TokenIssuer, PasswordVerifier
  application/services   LoginService (compara siempre la contraseña, aunque el usuario no coincida)
  infrastructure/token   JWT HS256 con stdlib (alg fijo, iss "agatha-platform", exp)
  infrastructure/security bcrypt; el hash viaja en base64 en el env
  infrastructure/http    POST /api/auth/login, GET /api/auth/me, límite de intentos
```

El middleware vive en `shared/http/auth.go` (igual que CORS) y el cableado en `internal/bootstrap/auth.go`. En `cmd/platform` queda `WithCORS(auth.Wrap(mux))`: **CORS afuera**, porque los preflight `OPTIONS` nunca traen credenciales.

## Endpoints

### `POST /api/auth/login` (público)

```json
{ "username": "admin", "password": "..." }
```

`200`:

```json
{ "accessToken": "eyJ...", "tokenType": "Bearer", "expiresAt": "2026-12-27T10:00:00Z", "username": "admin" }
```

| Status | `code` | Cuándo |
|---|---|---|
| 400 | `INVALID_INPUT` | JSON inválido, campos extra o más de 4 KB |
| 401 | `INVALID_CREDENTIALS` | Usuario o contraseña incorrectos (no dice cuál) |
| 429 | `TOO_MANY_ATTEMPTS` | 10 fallos en 15 min desde la misma IP; trae `Retry-After` (s) |
| 503 | `AUTH_NOT_CONFIGURED` | Faltan `AGATHA_ADMIN_*` / `AGATHA_JWT_SECRET` |

Respuesta con `Cache-Control: no-store`.

### `GET /api/auth/me` (admin o móvil)

`200 {"principal": "admin"}` o `{"principal": "mobile"}`. `401 UNAUTHORIZED` sin credencial válida. Sirve para validar al arrancar si el token guardado sigue vigente.

### Respuestas del middleware

| Status | `code` | Cuándo |
|---|---|---|
| 401 | `UNAUTHORIZED` | Sin credencial, key incorrecta, token inválido o vencido. Trae `WWW-Authenticate: Bearer` |
| 403 | `FORBIDDEN` | Key móvil válida en una ruta fuera de su alcance |

Las rutas inexistentes también responden 401 (no 404) a quien no se autentica.

## Alcance de la key móvil (`mobileRoutes`)

`GET /api/alerts/active`, `GET /api/alerts/pending`, `GET /api/alerts`, `GET /api/alerts/{id}`, `GET /api/alerts/devices/{id}/alerts`, `POST /api/alerts/{id}/classification`, `POST /api/alerts/{id}/label`, `POST /api/alerts/{id}/observations`, `PUT /api/devices/push-token`, `GET /api/dashboard/*` y `GET /api/auth/me`.

Fuera de alcance (403 con la key): umbrales del Rule Engine, políticas y exportación de alertas, stream SSE y eventos de prueba. La key vive en el APK y se puede extraer: este alcance es lo que limita el daño si se filtra.

## Variables de entorno

| Variable | Obligatoria | Descripción |
|---|---|---|
| `AGATHA_AUTH_MODE` | No | `off` (sin validar), `log` (valida y registra, NO bloquea; default) o `enforce` (401/403) |
| `AGATHA_ADMIN_USER` | Con `enforce` | Usuario web |
| `AGATHA_ADMIN_PASSWORD_HASH` | Con `enforce` | bcrypt (costo >= 10) en base64. Generar con `printf '%s' 'clave' \| go run ./cmd/auth -hash-password` |
| `AGATHA_JWT_SECRET` | Con `enforce` | >= 32 bytes. Generar con `go run ./cmd/auth -generate-key` |
| `AGATHA_JWT_TTL` | No | Vigencia de la sesión (`720h` por defecto; rango 1h a 2160h) |
| `AGATHA_MOBILE_API_KEYS` | No | Keys móviles separadas por comas (>= 32 caracteres cada una). Varias = rotación |
| `AGATHA_AUTH_CLIENT_IP_HEADER` | No | Encabezado con la IP real para el límite de login (p. ej. `X-Real-IP` del nginx). Solo si el backend no es alcanzable sin el proxy |

Las tres variables del admin van juntas o ninguna. Cualquier valor inválido detiene el arranque (exit 2).

**Por qué el hash va en base64**: el bcrypt lleva `$` (`$2a$12$...`) y tanto `godotenv` como el `env_file` de Docker Compose pueden interpretarlo como variable y romperlo en silencio.

## Despliegue sin romper la app ya instalada

1. Desplegar con `AGATHA_AUTH_MODE=log` y todas las variables. Nada se bloquea; cada petición sin credencial válida deja un `WARN "petición sin credencial válida"` con ruta y motivo (nunca la credencial ni la query).
2. Publicar el frontend con login y la versión de la app con la key.
3. Cuando los logs dejen de mostrar rechazos de clientes legítimos, pasar a `AGATHA_AUTH_MODE=enforce`.

## Rotar la key móvil

1. `AGATHA_MOBILE_API_KEYS=keyVieja,keyNueva` y reiniciar.
2. Publicar la app con `keyNueva`.
3. Cuando ya nadie use la versión vieja, dejar solo `keyNueva`.

## Limitaciones conocidas

- La key móvil no es un secreto real (se extrae del APK); es control de acceso básico más alcance limitado.
- El token web por query (`access_token`) puede quedar en logs de acceso del nginx: conviene no loguear la query de `/api/notifications/stream`.
- El límite de intentos es en memoria (se reinicia con el proceso) y, detrás del nginx sin `AGATHA_AUTH_CLIENT_IP_HEADER`, todas las peticiones comparten IP: 10 fallos bloquean el login para todos durante 15 min.
- Sin revocación individual de tokens: el único "logout global" es cambiar `AGATHA_JWT_SECRET`.

Guías de integración: `FRONTEND_AUTH_GUIDE.md` y `MOBILE_AUTH_GUIDE.md` en esta carpeta.
