import { describe, expect, it } from "vitest";
import { createDemoAlertStore, type DemoStreamEvent } from "./alertStore";
import { SCENARIO_POINT_ID } from "./demoData";
import { SCENARIO_STEPS, createDemoScenario } from "./scenario";

function setup() {
    const store = createDemoAlertStore({ now: () => new Date("2026-10-31T15:20:00Z") });
    const events: DemoStreamEvent[] = [];
    store.subscribe((event) => events.push(event));
    return { store, events, scenario: createDemoScenario(store) };
}

function activeLevel(store: ReturnType<typeof createDemoAlertStore>) {
    const [device] = store.getActive().devices;
    return device ? { level: device.level, blinking: device.blinking } : null;
}

describe("createDemoScenario", () => {
    it("recorre Amarilla → Naranja → Roja → clasificar → Normal en el Punto 1", () => {
        const { store, scenario } = setup();
        const seen: unknown[] = [];
        while (scenario.nextStep()) {
            scenario.advance();
            seen.push(activeLevel(store));
        }

        expect(seen).toEqual([
            { level: "AMARILLO", blinking: false },
            { level: "NARANJA", blinking: false },
            { level: "ROJO", blinking: true },
            { level: "ROJO", blinking: false },
            null,
        ]);
        expect(scenario.position()).toBe(SCENARIO_STEPS.length);
        expect(scenario.advance()).toBeNull();
        expect(store.search({ device: SCENARIO_POINT_ID, from: null, to: null, maxLevels: [], events: [], statuses: [] }, 1).items).toMatchObject([
            { code: "ALR-900001", state: "FINALIZADA", max_level: "ROJO", classification: "CONFIRMADA" },
        ]);
    });

    it("si la alerta ya se clasificó desde el modal, el paso de clasificar no hace nada", () => {
        const { store, events, scenario } = setup();
        scenario.advance();
        scenario.advance();
        scenario.advance();
        store.classify(store.activeAlertId()!, { classification: "FALSA_ALERTA", origin: "WEB" });
        events.length = 0;

        expect(scenario.advance()?.id).toBe("CLASIFICAR");
        expect(events).toEqual([]);
        expect(store.getDetail(store.activeAlertId()!).classification).toBe("FALSA_ALERTA");
    });

    it("reset borra las alertas y vuelve al primer paso", () => {
        const { store, scenario } = setup();
        scenario.advance();
        scenario.advance();
        scenario.reset();

        expect(scenario.position()).toBe(0);
        expect(store.getActive().devices).toEqual([]);
        scenario.advance();
        expect(store.getActive().devices[0].alerts[0].code).toBe("ALR-900001");
    });
});
