import { STREAM_EVENTS, type EventSourceLike } from "@/services/realtime";
import type { DemoAlertStore, DemoStreamEvent } from "./alertStore";

// Stream de alertas simulado con el comportamiento de GET /api/notifications/stream (NOTIFICATIONS_API.MD): `open`
// al conectar, cada evento con id `<epoch>-<seq>`, `ping` cada 25 s y reenvío de lo perdido con `?last_event_id=`
// (guarda los últimos 256). La caída se simula como un servidor que no responde: el navegador se rinde (CLOSED)
// y el cliente real reintenta con su backoff, igual que en producción.

const PING_INTERVAL_MS = 25_000;
/** Lo que tarda en "conectar": el indicador alcanza a mostrar el estado intermedio. */
const CONNECT_DELAY_MS = 300;
const REPLAY_BUFFER_SIZE = 256;

const CONNECTING = 0;
const OPEN = 1;
const CLOSED = 2;

const EVENT_NAME: Record<DemoStreamEvent["type"], string> = {
    changed: STREAM_EVENTS.changed,
    notification: STREAM_EVENTS.notification,
};

interface BufferedEvent {
    seq: number;
    event: DemoStreamEvent;
}

export interface DemoStreamController {
    /** Crea un `EventSource` simulado; se instala con `installEventSourceFactory`. */
    createEventSource(url: string): EventSourceLike;
    /** Simula la caída del servidor: corta las conexiones abiertas y rechaza las nuevas. */
    disconnect(): void;
    reconnect(): void;
    isConnected(): boolean;
}

function streamEvent(type: string, data?: string, lastEventId = ""): Event {
    return Object.assign(new Event(type), { data, lastEventId });
}

/** Secuencia del id `<epoch>-<seq>`; null si el id es de otro arranque o no se entiende. */
function parseSeq(lastEventId: string | null, epoch: number): number | null {
    const match = lastEventId?.match(/^(\d+)-(\d+)$/);
    return match && Number(match[1]) === epoch ? Number(match[2]) : null;
}

class DemoEventSource implements EventSourceLike {
    readyState = CONNECTING;
    private readonly target = new EventTarget();
    private readonly cleanups: (() => void)[] = [];

    addEventListener(type: string, listener: (event: Event) => void) {
        this.target.addEventListener(type, listener);
    }

    dispatch(event: Event) {
        if (this.readyState !== CLOSED) this.target.dispatchEvent(event);
    }

    onClose(cleanup: () => void) {
        this.cleanups.push(cleanup);
    }

    /** El servidor no responde: el navegador deja de reintentar y avisa con `error`. */
    fail() {
        if (this.readyState === CLOSED) return;
        this.release();
        this.target.dispatchEvent(streamEvent("error"));
    }

    close() {
        this.release();
    }

    private release() {
        this.readyState = CLOSED;
        this.cleanups.splice(0).forEach((cleanup) => cleanup());
    }
}

export function createDemoStreamController(store: DemoAlertStore, epoch = Date.now()): DemoStreamController {
    let connected = true;
    let seq = 0;
    const buffer: BufferedEvent[] = [];
    const sources = new Set<DemoEventSource>();

    // El "servidor" guarda los eventos aunque nadie esté conectado, como el backend: así una reconexión con
    // `last_event_id` recibe lo que pasó durante la caída.
    store.subscribe((event) => {
        const buffered = { seq: ++seq, event };
        buffer.push(buffered);
        if (buffer.length > REPLAY_BUFFER_SIZE) buffer.shift();
        for (const source of sources) {
            if (source.readyState === OPEN) deliver(source, buffered);
        }
    });

    function deliver(source: DemoEventSource, { seq: eventSeq, event }: BufferedEvent) {
        source.dispatch(streamEvent(EVENT_NAME[event.type], JSON.stringify(event.data), `${epoch}-${eventSeq}`));
    }

    function open(source: DemoEventSource, lastEventId: string | null) {
        if (source.readyState === CLOSED) return;
        if (!connected) {
            source.fail();
            return;
        }
        source.readyState = OPEN;
        source.dispatch(streamEvent("open"));
        const since = parseSeq(lastEventId, epoch);
        if (since !== null) {
            buffer.filter((buffered) => buffered.seq > since).forEach((buffered) => deliver(source, buffered));
        }
        const ping = setInterval(() => source.dispatch(streamEvent(STREAM_EVENTS.ping, "{}")), PING_INTERVAL_MS);
        source.onClose(() => clearInterval(ping));
    }

    return {
        createEventSource(url) {
            const source = new DemoEventSource();
            const lastEventId = new URL(url, "http://demo.local").searchParams.get("last_event_id");
            sources.add(source);
            source.onClose(() => sources.delete(source));
            const timer = setTimeout(() => open(source, lastEventId), CONNECT_DELAY_MS);
            source.onClose(() => clearTimeout(timer));
            return source;
        },
        disconnect() {
            connected = false;
            [...sources].forEach((source) => source.fail());
        },
        reconnect() {
            connected = true;
        },
        isConnected: () => connected,
    };
}
