import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { STREAM_EVENTS, createAlertStream, type AlertStreamHandlers } from "@/services/realtime";
import { createDemoAlertStore } from "./alertStore";
import { createDemoStreamController } from "./demoEventSource";

const EPOCH = 1761832800123;
const STREAM_URL = "/api/notifications/stream";

function setup() {
    const store = createDemoAlertStore();
    const controller = createDemoStreamController(store, EPOCH);
    return { store, controller };
}

function record(source: { addEventListener(type: string, listener: (event: Event) => void): void }) {
    const received: { type: string; id: string; data: unknown }[] = [];
    for (const type of ["open", "error", STREAM_EVENTS.changed, STREAM_EVENTS.notification, STREAM_EVENTS.ping]) {
        source.addEventListener(type, (event) => {
            const message = event as MessageEvent;
            received.push({ type, id: message.lastEventId ?? "", data: message.data ? JSON.parse(message.data) : undefined });
        });
    }
    return received;
}

beforeEach(() => {
    vi.useFakeTimers();
});

afterEach(() => {
    vi.useRealTimers();
});

describe("createDemoStreamController", () => {
    it("abre tras una espera corta y entrega los eventos con id <epoch>-<seq>", async () => {
        const { store, controller } = setup();
        const source = controller.createEventSource(STREAM_URL);
        const received = record(source);

        expect(source.readyState).toBe(0);
        await vi.advanceTimersByTimeAsync(300);
        expect(source.readyState).toBe(1);
        store.openAlert();

        expect(received.map(({ type, id }) => [type, id])).toEqual([
            ["open", ""],
            [STREAM_EVENTS.changed, `${EPOCH}-1`],
            [STREAM_EVENTS.notification, `${EPOCH}-2`],
        ]);
        expect(received[1].data).toMatchObject({ alert_code: "ALR-900001", change: "OPENED" });
    });

    it("manda un ping cada 25 s y deja de hacerlo al cerrar", async () => {
        const { controller } = setup();
        const source = controller.createEventSource(STREAM_URL);
        const received = record(source);
        await vi.advanceTimersByTimeAsync(300 + 50_000);
        source.close();
        await vi.advanceTimersByTimeAsync(50_000);

        expect(received.filter(({ type }) => type === STREAM_EVENTS.ping)).toHaveLength(2);
    });

    it("al caer avisa con error y CLOSED, y rechaza conexiones hasta que vuelve", async () => {
        const { controller } = setup();
        const source = controller.createEventSource(STREAM_URL);
        const received = record(source);
        await vi.advanceTimersByTimeAsync(300);

        controller.disconnect();
        expect(source.readyState).toBe(2);
        expect(received.at(-1)?.type).toBe("error");

        const retry = controller.createEventSource(STREAM_URL);
        const retryEvents = record(retry);
        await vi.advanceTimersByTimeAsync(300);
        expect(retryEvents.map(({ type }) => type)).toEqual(["error"]);

        controller.reconnect();
        const back = controller.createEventSource(STREAM_URL);
        const backEvents = record(back);
        await vi.advanceTimersByTimeAsync(300);
        expect(backEvents.map(({ type }) => type)).toEqual(["open"]);
        expect(controller.isConnected()).toBe(true);
    });

    it("reenvía lo ocurrido durante la caída a quien reconecta con last_event_id", async () => {
        const { store, controller } = setup();
        const first = record(controller.createEventSource(STREAM_URL));
        await vi.advanceTimersByTimeAsync(300);
        const id = store.openAlert()!.id;
        const lastId = first.at(-1)!.id;

        controller.disconnect();
        store.raiseLevel(id);
        controller.reconnect();
        const replayed = record(controller.createEventSource(`${STREAM_URL}?last_event_id=${lastId}`));
        await vi.advanceTimersByTimeAsync(300);

        expect(replayed.map(({ type, id: eventId }) => [type, eventId])).toEqual([
            ["open", ""],
            [STREAM_EVENTS.changed, `${EPOCH}-3`],
            [STREAM_EVENTS.notification, `${EPOCH}-4`],
        ]);
    });

    it("no reenvía nada con un id de otro arranque", async () => {
        const { store, controller } = setup();
        store.openAlert();
        const received = record(controller.createEventSource(`${STREAM_URL}?last_event_id=1-0`));
        await vi.advanceTimersByTimeAsync(300);

        expect(received.map(({ type }) => type)).toEqual(["open"]);
    });

    it("funciona con el cliente real del stream: conecta, recibe y se recupera de la caída", async () => {
        const { store, controller } = setup();
        const handlers = {
            onChanged: vi.fn<AlertStreamHandlers["onChanged"]>(),
            onNotification: vi.fn<AlertStreamHandlers["onNotification"]>(),
            onResync: vi.fn<AlertStreamHandlers["onResync"]>(),
            onStatusChange: vi.fn<AlertStreamHandlers["onStatusChange"]>(),
        };
        const stream = createAlertStream({ handlers, createEventSource: controller.createEventSource, random: () => 0 });

        stream.connect();
        await vi.advanceTimersByTimeAsync(300);
        expect(stream.status).toBe("live");
        const id = store.openAlert()!.id;
        expect(handlers.onNotification).toHaveBeenCalledWith(expect.objectContaining({ code: "OPEN" }));

        controller.disconnect();
        expect(stream.status).toBe("reconnecting");
        store.raiseLevel(id);
        controller.reconnect();
        await vi.advanceTimersByTimeAsync(1_000);

        expect(stream.status).toBe("live");
        expect(handlers.onNotification).toHaveBeenLastCalledWith(expect.objectContaining({ code: "UP_NARANJA_FAST" }));
        stream.close();
    });
});
