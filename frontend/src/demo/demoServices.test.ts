import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { isApiError } from "@/services/http/apiError";
import type { AlertSearchFilters } from "@/services/alertManagement";
import { createDemoAlertStore } from "./alertStore";
import { DEMO_POINTS, SCENARIO_POINT_ID } from "./demoData";
import { EXPORT_UNAVAILABLE_MESSAGE, createDemoAlertManagementService, createDemoDashboardService } from "./demoServices";

const NO_FILTERS: AlertSearchFilters = { device: null, from: null, to: null, maxLevels: [], events: [], statuses: [] };

beforeEach(() => {
    vi.useFakeTimers();
});

afterEach(() => {
    vi.useRealTimers();
});

/** Resuelve la promesa avanzando el reloj falso; devuelve el valor o el error. */
async function settle<T>(promise: Promise<T>): Promise<{ value?: T; error?: unknown }> {
    const outcome = promise.then(
        (value) => ({ value }),
        (error: unknown) => ({ error })
    );
    await vi.advanceTimersByTimeAsync(1_000);
    return outcome;
}

describe("createDemoAlertManagementService", () => {
    it("responde del store tras una espera corta", async () => {
        const store = createDemoAlertStore();
        const service = createDemoAlertManagementService(store);
        store.openAlert();

        let resolved = false;
        const pending = service.getPending().then((response) => {
            resolved = true;
            return response;
        });
        await vi.advanceTimersByTimeAsync(100);
        expect(resolved).toBe(false);

        expect((await settle(pending)).value?.total).toBe(1);
    });

    it("clasificar responde como el backend y deja la alerta clasificada", async () => {
        const store = createDemoAlertStore();
        const service = createDemoAlertManagementService(store);
        const id = store.openAlert("ROJO")!.id;

        const { value } = await settle(service.classify(id, { classification: "CONFIRMADA", origin: "WEB" }));

        expect(value).toMatchObject({ outcome: "SAVED", alert: { blinking: false } });
        expect((await settle(service.search(NO_FILTERS, 1))).value?.items[0].classification).toBe("CONFIRMADA");
    });

    it("los errores del store llegan como ApiError", async () => {
        const store = createDemoAlertStore();
        const service = createDemoAlertManagementService(store);
        const id = store.openAlert()!.id;

        const { error } = await settle(service.label(id, { label: "MOVIMIENTO_TIERRA", origin: "WEB" }));

        expect(isApiError(error) && error.status).toBe(409);
    });

    it("la exportación se rechaza con un mensaje para el usuario", async () => {
        const service = createDemoAlertManagementService(createDemoAlertStore());

        const { error } = await settle(service.exportXlsx(NO_FILTERS));

        expect(isApiError(error) && error.message).toBe(EXPORT_UNAVAILABLE_MESSAGE);
    });

    it("cancela con AbortError, como fetch", async () => {
        const service = createDemoAlertManagementService(createDemoAlertStore());
        const controller = new AbortController();

        const result = settle(service.getActive(controller.signal));
        controller.abort();

        expect(((await result).error as DOMException).name).toBe("AbortError");
        expect(((await settle(service.getActive(controller.signal))).error as DOMException).name).toBe("AbortError");
    });
});

describe("createDemoDashboardService", () => {
    const now = new Date("2026-10-31T15:00:00Z");

    it("lista los tres puntos conectados ahora", async () => {
        const service = createDemoDashboardService(createDemoAlertStore(), () => now);

        const { value } = await settle(service.getDevices());

        expect(value?.map((device) => device.id)).toEqual(Object.values(DEMO_POINTS).map((point) => point.id));
        expect(value?.every((device) => device.lastConnection === now.toISOString())).toBe(true);
    });

    it("las series del punto siguen sus alertas", async () => {
        const store = createDemoAlertStore({ now: () => now });
        const service = createDemoDashboardService(store, () => now);
        store.openAlert("ROJO");
        const range = ["2026-10-31T14:00:00.000Z", "2026-10-31T15:00:00.000Z"] as const;

        const alert = await settle(service.getSeries(SCENARIO_POINT_ID, ...range, ["indicador"]));
        const quiet = await settle(service.getSeries(DEMO_POINTS.punto2.id, ...range, ["indicador"]));

        expect(alert.value?.series[0].data.at(-1)?.value).toBeGreaterThan(0.35);
        expect(quiet.value?.series[0].data.at(-1)?.value).toBeLessThan(0.16);
    });
});
