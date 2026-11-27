const STORAGE_KEY = "agatha.session";

/**
 * Sesión del admin. Va en `localStorage` porque tiene que sobrevivir al cierre del navegador (30 días) y
 * compartirse entre pestañas. El costo es que un XSS puede leer el token: no usar `dangerouslySetInnerHTML`
 * con datos del backend. Una cookie `HttpOnly` sería de terceros (front en Vercel, API en otro dominio).
 */
export interface StoredSession {
    accessToken: string;
    expiresAt: string; // ISO 8601
    username: string;
}

type Listener = (session: StoredSession | null) => void;
const listeners = new Set<Listener>();

/** `localStorage`, o null si no existe o el navegador lo bloquea (Safari privado lanza `SecurityError`). */
function storage(): Storage | null {
    try {
        return globalThis.localStorage ?? null;
    } catch {
        return null;
    }
}

function notify(session: StoredSession | null) {
    listeners.forEach((listener) => listener(session));
}

export const tokenStore = {
    get(): StoredSession | null {
        const raw = storage()?.getItem(STORAGE_KEY);
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
        storage()?.setItem(STORAGE_KEY, JSON.stringify(session));
        notify(session);
    },
    clear() {
        storage()?.removeItem(STORAGE_KEY);
        notify(null);
    },
    subscribe(listener: Listener): () => void {
        listeners.add(listener);
        return () => {
            listeners.delete(listener);
        };
    },
};

// Otra pestaña cerró sesión o inició sesión: propagar.
if (typeof window !== "undefined") {
    window.addEventListener("storage", (event) => {
        if ((event as StorageEvent).key === STORAGE_KEY) notify(tokenStore.get());
    });
}
