import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createAlertStream, parseEventData, reconnectDelay, streamUrl, type EventSourceLike } from "./alertStream.client";
import type { AlertStreamHandlers } from "./alertStream.types";

const CONNECTING = 0;
const CLOSED = 2;

/** EventSource falso: las pruebas disparan sus eventos a mano. */
class FakeEventSource implements EventSourceLike {
    readyState = CONNECTING;
    closed = false;
    readonly url: string;
    private listeners = new Map<string, ((event: Event) => void)[]>();

    constructor(url: string) {
        this.url = url;
    }

    addEventListener(type: string, listener: (event: Event) => void) {
        this.listeners.set(type, [...(this.listeners.get(type) ?? []), listener]);
    }

    close() {
        this.closed = true;
        this.readyState = CLOSED;
    }

    emit(type: string, init: { data?: string; lastEventId?: string } = {}) {
        const event = Object.assign(new Event(type), { data: init.data, lastEventId: init.lastEventId ?? "" });
        this.listeners.get(type)?.forEach((listener) => listener(event));
    }

    /** El navegador dejó de reintentar (p. ej. el servidor respondió 503). */
    giveUp() {
        this.readyState = CLOSED;
        this.emit("error");
    }
}

let sources: FakeEventSource[];
let handlers: { [K in keyof AlertStreamHandlers]: ReturnType<typeof vi.fn> };

const latest = () => sources.at(-1)!;

function startStream() {
    const stream = createAlertStream({
        handlers: handlers as unknown as AlertStreamHandlers,
        createEventSource: (url) => {
            const source = new FakeEventSource(url);
            sources.push(source);
            return source;
        },
        random: () => 1,
    });
    stream.connect();
    return stream;
}

beforeEach(() => {
    vi.useFakeTimers();
    sources = [];
    handlers = { onChanged: vi.fn(), onNotification: vi.fn(), onResync: vi.fn(), onStatusChange: vi.fn() };
});

afterEach(() => {
    vi.useRealTimers();
});

describe("reconnectDelay", () => {
    it("crece exponencialmente con jitter y no pasa de 30 s", () => {
        expect(reconnectDelay(0, () => 0)).toBe(500);
        expect(reconnectDelay(0, () => 1)).toBe(1_000);
        expect(reconnectDelay(3, () => 1)).toBe(8_000);
        expect(reconnectDelay(20, () => 1)).toBe(30_000);
    });
});

describe("streamUrl", () => {
    it("agrega el último id solo si existe", () => {
        expect(streamUrl(null)).toBe("/api/notifications/stream");
        expect(streamUrl("1761832800123-42")).toBe("/api/notifications/stream?last_event_id=1761832800123-42");
    });
});

describe("parseEventData", () => {
    it("devuelve el objeto si trae el id numérico", () => {
        expect(parseEventData<{ alert_id: number }>('{"alert_id":142}', "alert_id")).toEqual({ alert_id: 142 });
    });

    it.each(["no es json", "null", '{"alert_id":"142"}', "[]"])("ignora %s", (data) => {
        expect(parseEventData<{ alert_id: number }>(data, "alert_id")).toBeNull();
    });
});

describe("createAlertStream", () => {
    it("al conectar pasa a live y pide resincronizar", () => {
        const stream = startStream();
        expect(handlers.onStatusChange).toHaveBeenLastCalledWith("connecting");

        latest().emit("open");

        expect(stream.status).toBe("live");
        expect(handlers.onResync).toHaveBeenCalledTimes(1);
    });

    it("entrega los eventos ya parseados e ignora los que no se entienden", () => {
        startStream();
        latest().emit("open");

        latest().emit("alert.changed", { data: '{"alert_id":142,"change":"OPENED"}', lastEventId: "e-1" });
        latest().emit("alert.notification", { data: '{"message_id":9,"alert_id":142}', lastEventId: "e-2" });
        latest().emit("alert.changed", { data: "roto", lastEventId: "e-3" });
        latest().emit("resync");

        expect(handlers.onChanged).toHaveBeenCalledExactlyOnceWith({ alert_id: 142, change: "OPENED" });
        expect(handlers.onNotification).toHaveBeenCalledExactlyOnceWith({ message_id: 9, alert_id: 142 });
        expect(handlers.onResync).toHaveBeenCalledTimes(2);
    });

    it("si el navegador se rinde, reabre tras el backoff con el último id", () => {
        const stream = startStream();
        latest().emit("open");
        latest().emit("alert.changed", { data: '{"alert_id":1}', lastEventId: "e-7" });

        latest().giveUp();
        expect(stream.status).toBe("reconnecting");
        expect(sources).toHaveLength(1);

        vi.advanceTimersByTime(1_000);
        expect(sources).toHaveLength(2);
        expect(latest().url).toBe("/api/notifications/stream?last_event_id=e-7");

        latest().emit("open");
        expect(stream.status).toBe("live");
        expect(handlers.onResync).toHaveBeenCalledTimes(2);
    });

    it("no abre otra conexión mientras el navegador reintenta solo", () => {
        const stream = startStream();
        latest().emit("open");

        latest().emit("error"); // readyState sigue en CONNECTING

        vi.advanceTimersByTime(30_000);
        expect(sources).toHaveLength(1);
        expect(stream.status).toBe("reconnecting");
    });

    it("el ping posterga el watchdog; sin actividad en 60 s reconecta", () => {
        startStream();
        latest().emit("open");

        vi.advanceTimersByTime(50_000);
        latest().emit("ping");
        vi.advanceTimersByTime(50_000);
        expect(sources).toHaveLength(1);

        vi.advanceTimersByTime(10_000);
        expect(sources[0].closed).toBe(true);
        vi.advanceTimersByTime(1_000);
        expect(sources).toHaveLength(2);
    });

    it("pasa a offline tras tres fallas seguidas y vuelve a live al conectar", () => {
        const stream = startStream();

        latest().giveUp();
        expect(stream.status).toBe("connecting");
        vi.advanceTimersByTime(1_000);
        latest().giveUp();
        vi.advanceTimersByTime(2_000);
        latest().giveUp();
        expect(stream.status).toBe("offline");

        vi.advanceTimersByTime(4_000);
        latest().emit("open");
        expect(stream.status).toBe("live");
    });

    it("close cancela los reintentos e ignora los eventos de la conexión cerrada", () => {
        const stream = startStream();
        const first = latest();
        first.emit("open");
        first.giveUp();

        stream.close();
        vi.advanceTimersByTime(60_000);
        first.emit("alert.changed", { data: '{"alert_id":1}' });

        expect(stream.status).toBe("idle");
        expect(sources).toHaveLength(1);
        expect(handlers.onChanged).not.toHaveBeenCalled();
    });
});

describe("installEventSourceFactory", () => {
    it("lo usan los streams que se abren sin un EventSource propio", async () => {
        // Módulo nuevo: la fábrica instalada es global y no debe afectar a las demás pruebas.
        vi.resetModules();
        const client = await import("./alertStream.client");
        const created: string[] = [];
        client.installEventSourceFactory((url) => {
            created.push(url);
            return new FakeEventSource(url);
        });

        client.createAlertStream({ handlers: handlers as unknown as AlertStreamHandlers }).connect();

        expect(created).toEqual(["/api/notifications/stream"]);
    });
});
