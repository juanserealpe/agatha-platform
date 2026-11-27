# Guía de integración web: login y autenticación

Para: quien implemente el login en `frontend/` (React 19 + React Router 7 + TanStack Query).
Contrato completo del backend: `backend/internal/modules/auth/README.md`.

## 1. Qué hay que construir

1. Pantalla de login (usuario + contraseña) que llama `POST /api/auth/login`.
2. Guardar el token y mandarlo en **todas** las peticiones como `Authorization: Bearer <token>`.
3. Mandarlo también al stream SSE, pero como `?access_token=<token>` (EventSource no permite encabezados).
4. Ante un `401`, borrar el token y llevar al login.
5. Proteger las rutas de la app: sin sesión, redirigir a `/login`.
6. Botón "Cerrar sesión" (solo borra el token local).

Hay **un solo usuario** (admin) y la sesión dura 30 días por defecto. No hay refresh token ni registro de usuarios.

## 2. Contrato

### Login

```http
POST /api/auth/login
Content-Type: application/json

{ "username": "admin", "password": "..." }
```

`200`:

```json
{
  "accessToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "tokenType": "Bearer",
  "expiresAt": "2026-12-27T10:00:00Z",
  "username": "admin"
}
```

| Status | `code` | Qué mostrar |
|---|---|---|
| 400 | `INVALID_INPUT` | "Complete usuario y contraseña." |
| 401 | `INVALID_CREDENTIALS` | "Usuario o contraseña incorrectos." |
| 429 | `TOO_MANY_ATTEMPTS` | "Demasiados intentos. Intente de nuevo en N minutos." (N sale de `Retry-After`, en segundos) |
| 503 | `AUTH_NOT_CONFIGURED` | "El inicio de sesión no está disponible. Contacte al administrador." |

El body tiene que tener **exactamente** `username` y `password`: un campo extra da `400`.

### Verificar la sesión guardada

```http
GET /api/auth/me
Authorization: Bearer <token>
```

`200 {"principal": "admin"}` o `401`. Usarlo al arrancar la app si hay un token guardado.

### Cualquier otro endpoint

| Status | `code` | Significado |
|---|---|---|
| 401 | `UNAUTHORIZED` | Sin token, token inválido o **vencido**. Ir al login. |
| 403 | `FORBIDDEN` | No le pasa a la web (es para la key móvil fuera de alcance). Tratarlo como error genérico. |

## 3. Dónde tocar el código

### 3.1 Almacén del token: `src/services/auth/tokenStore.ts` (nuevo)

```ts
const STORAGE_KEY = "agatha.session";

export interface StoredSession {
    accessToken: string;
    expiresAt: string; // ISO 8601
    username: string;
}

type Listener = (session: StoredSession | null) => void;
const listeners = new Set<Listener>();

export const tokenStore = {
    get(): StoredSession | null {
        const raw = localStorage.getItem(STORAGE_KEY);
        if (!raw) return null;
        try {
            const session = JSON.parse(raw) as StoredSession;
            // Vencido localmente: no esperar al 401.
            if (Date.parse(session.expiresAt) <= Date.now()) {
                tokenStore.clear();
                return null;
            }
            return session;
        } catch {
            tokenStore.clear();
            return null;
        }
    },
    accessToken(): string | null {
        return tokenStore.get()?.accessToken ?? null;
    },
    set(session: StoredSession) {
        localStorage.setItem(STORAGE_KEY, JSON.stringify(session));
        listeners.forEach((l) => l(session));
    },
    clear() {
        localStorage.removeItem(STORAGE_KEY);
        listeners.forEach((l) => l(null));
    },
    subscribe(listener: Listener): () => void {
        listeners.add(listener);
        return () => listeners.delete(listener);
    },
};

// Otra pestaña cerró sesión o inició sesión: propagar.
window.addEventListener("storage", (event) => {
    if (event.key === STORAGE_KEY) listeners.forEach((l) => l(tokenStore.get()));
});
```

**Por qué `localStorage`**: la sesión tiene que sobrevivir al cierre del navegador (30 días) y compartirse entre pestañas. El costo es que un XSS puede leer el token. Mitigación: no usar `dangerouslySetInnerHTML` con datos del backend y tener una CSP en Vercel. Una cookie `HttpOnly` sería más segura, pero con el front en Vercel y la API en otro dominio sería una cookie de terceros, y Safari las bloquea.

### 3.2 Cliente HTTP: `src/services/http/httpClient.ts`

`buildHeaders` ya es el único punto donde se arman las cabeceras (el comentario dice "aquí se agregará el token"). Cambios:

```ts
import { tokenStore } from "@/services/auth/tokenStore";

/** Se emite cuando el backend responde 401: la sesión ya no sirve. */
export const SESSION_EXPIRED_EVENT = "agatha:session-expired";

function buildHeaders(hasBody: boolean): HeadersInit {
    const headers: Record<string, string> = { Accept: "application/json" };
    if (hasBody) headers["Content-Type"] = "application/json";
    const token = tokenStore.accessToken();
    if (token) headers.Authorization = `Bearer ${token}`;
    return headers;
}
```

Y en `send`, antes del `throw await toApiError(...)`:

```ts
const LOGIN_PATH = "/api/auth/login";

if (response.status === 401 && path !== LOGIN_PATH) {
    tokenStore.clear();
    window.dispatchEvent(new Event(SESSION_EXPIRED_EVENT));
}
if (!response.ok) throw await toApiError(response, options.errorMessage);
```

El `401` del propio login **no** cierra la sesión: es "contraseña incorrecta".

Si `send` solo acepta `"GET" | "POST"`, el login usa `POST`, así que alcanza.

### 3.3 Servicio de auth: `src/services/auth/auth.service.ts` (nuevo)

```ts
import { httpClient, apiPath } from "@/services/http";
import { tokenStore, type StoredSession } from "./tokenStore";

interface LoginResponse {
    accessToken: string;
    tokenType: "Bearer";
    expiresAt: string;
    username: string;
}

export async function login(username: string, password: string): Promise<StoredSession> {
    const { data } = await httpClient.post<LoginResponse>(
        apiPath("api", "auth", "login"),
        { username, password },
        { errorMessage: "No fue posible iniciar sesión." },
    );
    const session = { accessToken: data.accessToken, expiresAt: data.expiresAt, username: data.username };
    tokenStore.set(session);
    return session;
}

export async function verifySession(): Promise<boolean> {
    if (!tokenStore.accessToken()) return false;
    try {
        await httpClient.get<{ principal: string }>(apiPath("api", "auth", "me"));
        return true;
    } catch {
        return false; // el 401 ya limpió el token en httpClient
    }
}

export function logout() {
    tokenStore.clear();
}
```

Para el `429`, `ApiError` no expone encabezados. Opciones: mostrar un mensaje sin minutos ("Intente más tarde"), o agregar `retryAfter` a `ApiError` leyendo `response.headers.get("Retry-After")` en `toApiError`. La primera alcanza.

### 3.4 Stream SSE: `src/services/realtime/alertStream.client.ts`

`streamUrl` arma la URL en cada conexión, así que basta con agregar el token:

```ts
import { tokenStore } from "@/services/auth/tokenStore";

export function streamUrl(lastEventId: string | null): string {
    return apiUrl(STREAM_PATH, {
        last_event_id: lastEventId,
        access_token: tokenStore.accessToken(),
    });
}
```

(`buildQueryString` ya omite los valores `null`.)

**Token vencido en el stream**: con un `401`, `EventSource` dispara `error` y queda en `readyState === CLOSED` (el cliente ya contempla `READY_STATE_CLOSED`). En ese caso, antes de reintentar, llamar `verifySession()`. Si da `false`, no reconectar: el evento `SESSION_EXPIRED_EVENT` ya llevó al login. Si da `true`, era otra falla y sigue el backoff normal.

**No abrir el stream sin sesión.** Cerrarlo en `logout` (`stream.close()`).

**Actualizar los tests** de `alertStream.client.test.ts` que comparan la URL exacta de `streamUrl`.

### 3.5 Rutas: `src/app/router.tsx`

La demo usa **el mismo** `createAppRouter` montado con `basename: "/demo"`. Por eso el guard se activa con una opción, no con una ruta aparte:

```tsx
export interface AppRouterOptions {
    basename?: string;
    /** false en la demo: no hay backend real ni login. */
    requireSession?: boolean;
}

export function createAppRouter({ basename, requireSession = true }: AppRouterOptions = {}) {
    const layout = requireSession ? <RequireSession><RootLayout /></RequireSession> : <RootLayout />;
    return createBrowserRouter(
        [
            ...(requireSession ? [{ path: ROUTES.login, element: <LoginPage /> }] : []),
            {
                path: "/",
                element: layout,
                children: [ /* igual que hoy */ ],
            },
        ],
        { basename },
    );
}
```

Agregar `login: "/login"` a `ROUTES`, y que el punto donde se arma el router de la demo pase `requireSession: false`.

`RequireSession`:

```tsx
function RequireSession({ children }: { children: React.ReactNode }) {
    const [session, setSession] = useState(() => tokenStore.get());
    const location = useLocation();

    useEffect(() => tokenStore.subscribe(setSession), []);

    if (!session) return <Navigate to={ROUTES.login} replace state={{ from: location.pathname }} />;
    return <>{children}</>;
}
```

Al arrancar, si hay token, llamar `verifySession()` una vez (por ejemplo en `RootLayout`) para detectar un secreto rotado en el servidor.

Escuchar `SESSION_EXPIRED_EVENT` en un componente alto (dentro del router, para tener `navigate`):

```tsx
useEffect(() => {
    const onExpired = () => {
        queryClient.clear(); // no dejar datos del admin en caché
        navigate(ROUTES.login, { replace: true });
    };
    window.addEventListener(SESSION_EXPIRED_EVENT, onExpired);
    return () => window.removeEventListener(SESSION_EXPIRED_EVENT, onExpired);
}, [navigate, queryClient]);
```

En `LoginPage`, después de un login correcto: `navigate(location.state?.from ?? ROUTES.monitoring, { replace: true })`. Si ya hay sesión al entrar a `/login`, redirigir directo.

### 3.6 TanStack Query

El `retry` global de `src/main.tsx` ya usa `!isClientError(error)`, así que los `401` no se reintentan. No hay que tocarlo.

En `logout`: `queryClient.clear()` además de `tokenStore.clear()`.

### 3.7 Modo demo

La demo bloquea el backend con `blockBackendRequests` y usa un `EventSource` simulado. Con `requireSession: false` (§3.5) sigue funcionando sin login. Verificar que `verifySession()` y el listener de `SESSION_EXPIRED_EVENT` tampoco corran en la demo.

## 4. Configuración

- **Desarrollo**: el proxy de Vite reenvía `/api` al backend (`localApiUrl` o, si está vacío, el VPS), así que `/api/auth/*` funciona sin cambios. Ojo: si `/api/notifications` apunta a otro backend local (`localNotificationsUrl`), ese backend tiene que tener el **mismo** `AGATHA_JWT_SECRET`, o el stream va a dar `401`. Con `AGATHA_AUTH_MODE=log` en el backend se puede seguir trabajando sin login mientras se implementa.
- **Producción**: `VITE_API_BASE_URL` igual que hoy. El origen de Vercel tiene que estar en `AGATHA_CORS_ALLOWED_ORIGINS` del backend (CORS ya permite `Authorization`).
- **No guardar** usuario ni contraseña en variables `VITE_*`: se compilan dentro del bundle público.

## 5. Checklist

- [ ] `tokenStore` con vencimiento local y sincronización entre pestañas.
- [ ] `Authorization: Bearer` en `buildHeaders`.
- [ ] `401` (salvo en el login) limpia el token y emite `SESSION_EXPIRED_EVENT`.
- [ ] Pantalla de login con los mensajes de la tabla del §2 (400/401/429/503).
- [ ] `access_token` en `streamUrl`; stream cerrado sin sesión; `verifySession()` antes de reconectar tras `CLOSED`.
- [ ] `RequireSession` en todas las rutas salvo `/login`; `requireSession: false` en la demo.
- [ ] `verifySession()` al arrancar con un token guardado.
- [ ] Cerrar sesión: limpia token, caché de Query y stream.
- [ ] Tests actualizados (`httpClient.test.ts`, `alertStream.client.test.ts`) y nuevos para `tokenStore` y el guard.

## 6. Probar contra el backend local

```bash
# Login
curl -s -X POST http://localhost:8080/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"<clave>"}'

# Con el token
curl -s http://localhost:8080/api/auth/me -H "Authorization: Bearer <token>"

# Stream
curl -N "http://localhost:8080/api/notifications/stream?access_token=<token>"
```

Para ver los `401` reales, el backend tiene que estar con `AGATHA_AUTH_MODE=enforce`. En `log` todo pasa y los rechazos solo aparecen en el log del servidor.
