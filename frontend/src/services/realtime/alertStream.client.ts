import { tokenStore } from "@/services/auth/tokenStore";
import { apiPath, apiUrl } from "@/services/http";
import {
    STREAM_EVENTS,
    type AlertChangedEvent,
    type AlertNotificationEvent,
    type AlertStreamHandlers,
    type StreamStatus,
} from "./alertStream.types";

const STREAM_PATH = apiPath("api", "notifications", "stream");

/** Sin ningún evento (ni el `ping`, que llega cada 25 s) durante este tiempo, la conexión se da por colgada. */
const WATCHDOG_MS = 60_000;
const BACKOFF_BASE_MS = 1_000;
const BACKOFF_MAX_MS = 30_000;
/** Fallas seguidas a partir de las cuales el estado pasa a `offline`. */
const OFFLINE_AFTER_FAILURES = 3;
/** `EventSource.CLOSED`: el navegador dejó de reintentar (p. ej. tras un 404 o un 503). */
const READY_STATE_CLOSED = 2;

/** Lo que el cliente usa de `EventSource`; las pruebas lo sustituyen por uno falso. */
export interface EventSourceLike {
    readonly readyState: number;
    addEventListener(type: string, listener: (event: Event) => void): void;
    close(): void;
}

let defaultCreateEventSource: (url: string) => EventSourceLike = (url) => new EventSource(url);

/**
 * Reemplaza el `EventSource` de todos los streams que se abran sin uno propio (la demo usa uno simulado).
 * Solo antes del primer render: no se cambia en caliente.
 */
export function installEventSourceFactory(factory: (url: string) => EventSourceLike) {
    defaultCreateEventSource = factory;
}

export interface AlertStreamOptions {
    handlers: AlertStreamHandlers;
    createEventSource?: (url: string) => EventSourceLike;
    random?: () => number;
    /**
     * Se consulta cuando el navegador se rinde (`CLOSED`), antes de reintentar: con un token vencido el
     * backend responde 401 y `EventSource` no deja ver el status. `false` = la sesión ya no sirve, no se
     * reconecta. Sin esta opción siempre se reintenta (la demo no tiene sesión).
     */
    shouldReconnect?: () => Promise<boolean>;
}

export interface AlertStream {
    /** Abre la conexión; no hace nada si ya está abierta o reintentando. */
    connect(): void;
    /** Cierra la conexión y cancela los reintentos. Se puede volver a abrir con `connect`. */
    close(): void;
    readonly status: StreamStatus;
}

/**
 * Espera antes del reintento número `attempt` (desde 0): exponencial con tope de 30 s y jitter, para que
 * todas las pestañas no reconecten al mismo tiempo cuando el servidor vuelve.
 */
export function reconnectDelay(attempt: number, random: () => number = Math.random): number {
    const cap = Math.min(BACKOFF_MAX_MS, BACKOFF_BASE_MS * 2 ** attempt);
    return cap / 2 + random() * (cap / 2);
}

/**
 * URL del stream. Con el último id recibido el servidor reenvía lo que se perdió: un `EventSource` nuevo
 * no manda la cabecera `Last-Event-ID`, así que va como parámetro. El token también va como parámetro
 * (`EventSource` no permite encabezados) y se lee en cada conexión.
 */
export function streamUrl(lastEventId: string | null): string {
    return apiUrl(STREAM_PATH, {
        last_event_id: lastEventId,
        access_token: tokenStore.accessToken(),
    });
}

/** JSON de un evento; null si no se entiende. Se ignora: la próxima resincronización corrige la caché. */
export function parseEventData<T>(data: unknown, idField: keyof T & string): T | null {
    if (typeof data !== "string") return null;
    try {
        const value: unknown = JSON.parse(data);
        if (typeof value === "object" && value !== null && typeof (value as Record<string, unknown>)[idField] === "number") {
            return value as T;
        }
    } catch {
        // JSON inválido.
    }
    return null;
}

/**
 * Cliente del stream de alertas sobre `EventSource`. Además de la reconexión nativa del navegador:
 * - Si el navegador se rinde (`CLOSED`), abre una conexión nueva con backoff y `?last_event_id=`.
 * - Watchdog: sin actividad en 60 s (conexión colgada tras suspender el equipo o cambiar de red), reconecta.
 * - Pide resincronizar por REST en cada conexión, porque el stream complementa la API y no la reemplaza.
 */
export function createAlertStream({
    handlers,
    createEventSource = defaultCreateEventSource,
    random = Math.random,
    shouldReconnect,
}: AlertStreamOptions): AlertStream {
    let source: EventSourceLike | null = null;
    let status: StreamStatus = "idle";
    let lastEventId: string | null = null;
    let failures = 0;
    let hasConnected = false;
    let watchdog: ReturnType<typeof setTimeout> | undefined;
    let retry: ReturnType<typeof setTimeout> | undefined;
    /** Verificando la sesión antes de reconectar; `close` lo invalida. */
    let checking: object | null = null;

    function setStatus(next: StreamStatus) {
        if (next === status) return;
        status = next;
        handlers.onStatusChange(next);
    }

    function armWatchdog() {
        clearTimeout(watchdog);
        watchdog = setTimeout(() => fail("hung"), WATCHDOG_MS);
    }

    /** Actividad de un evento con datos: posterga el watchdog y guarda el id para el reenvío. */
    function track(event: Event) {
        armWatchdog();
        const id = (event as MessageEvent).lastEventId;
        if (id) lastEventId = id;
    }

    function scheduleReconnect() {
        retry = setTimeout(() => {
            retry = undefined;
            open();
        }, reconnectDelay(failures - 1, random));
    }

    /**
     * Una falla. `"keep"`: el navegador ya está reintentando solo (con `Last-Event-ID`) y el watchdog cubre el
     * caso de que nunca lo logre. `"hung"` (watchdog) o `"closed"` (el navegador se rindió): se descarta la
     * conexión y se abre otra tras el backoff; con `"closed"` puede ser un 401, así que antes se verifica la
     * sesión si hay `shouldReconnect`.
     */
    function fail(kind: "keep" | "hung" | "closed") {
        failures += 1;
        setStatus(failures >= OFFLINE_AFTER_FAILURES ? "offline" : hasConnected ? "reconnecting" : "connecting");
        if (kind === "keep") return;

        clearTimeout(watchdog);
        source?.close();
        source = null;
        if (kind === "hung" || !shouldReconnect) {
            scheduleReconnect();
            return;
        }

        const check = {};
        checking = check;
        shouldReconnect()
            // Si la verificación misma falla, era otra falla: sigue el backoff normal.
            .catch(() => true)
            .then((ok) => {
                if (checking !== check) return; // se cerró mientras tanto
                checking = null;
                if (ok) scheduleReconnect();
            });
    }

    function open() {
        const current = createEventSource(streamUrl(lastEventId));
        source = current;
        armWatchdog();

        // Los eventos de una conexión ya descartada no tocan el estado.
        const on = (type: string, listener: (event: Event) => void) =>
            current.addEventListener(type, (event) => {
                if (source === current) listener(event);
            });

        on("open", () => {
            failures = 0;
            hasConnected = true;
            armWatchdog();
            setStatus("live");
            handlers.onResync();
        });
        on("error", () => fail(current.readyState === READY_STATE_CLOSED ? "closed" : "keep"));
        on(STREAM_EVENTS.changed, (event) => {
            track(event);
            const data = parseEventData<AlertChangedEvent>((event as MessageEvent).data, "alert_id");
            if (data) handlers.onChanged(data);
        });
        on(STREAM_EVENTS.notification, (event) => {
            track(event);
            const data = parseEventData<AlertNotificationEvent>((event as MessageEvent).data, "message_id");
            if (data) handlers.onNotification(data);
        });
        on(STREAM_EVENTS.resync, () => {
            armWatchdog();
            handlers.onResync();
        });
        on(STREAM_EVENTS.ping, armWatchdog);
    }

    return {
        connect() {
            if (source || retry || checking) return;
            setStatus(hasConnected ? "reconnecting" : "connecting");
            open();
        },
        close() {
            clearTimeout(watchdog);
            clearTimeout(retry);
            retry = undefined;
            checking = null;
            source?.close();
            source = null;
            failures = 0;
            setStatus("idle");
        },
        get status() {
            return status;
        },
    };
}
