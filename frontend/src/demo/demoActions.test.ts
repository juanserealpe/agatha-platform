import { describe, expect, it } from "vitest";
import { createDemoAlertStore } from "./alertStore";
import { alertToClassify, alertToRaise, pointForNewAlert } from "./demoActions";
import { DEMO_POINTS } from "./demoData";

function setup() {
    let clock = new Date("2026-10-31T15:20:00Z").getTime();
    // Cada consulta del reloj avanza un minuto: las alertas se abren en momentos distintos.
    const store = createDemoAlertStore({ now: () => new Date((clock += 60_000)) });
    return store;
}

describe("pointForNewAlert", () => {
    it("usa los puntos 2 y 3 antes que el Punto 1 del escenario", () => {
        const store = setup();
        expect(pointForNewAlert(store)).toBe(DEMO_POINTS.punto2.id);
        store.openAlert("AMARILLO", DEMO_POINTS.punto2.id);
        expect(pointForNewAlert(store)).toBe(DEMO_POINTS.punto3.id);
        store.openAlert("AMARILLO", DEMO_POINTS.punto3.id);
        expect(pointForNewAlert(store)).toBe(DEMO_POINTS.punto1.id);
    });

    it("devuelve null si los tres puntos tienen una alerta activa", () => {
        const store = setup();
        for (const point of Object.values(DEMO_POINTS)) store.openAlert("AMARILLO", point.id);
        expect(pointForNewAlert(store)).toBeNull();
    });

    it("vuelve a ofrecer un punto cuando su alerta termina", () => {
        const store = setup();
        for (const point of Object.values(DEMO_POINTS)) store.openAlert("AMARILLO", point.id);
        store.finishAlert(store.activeAlertId(DEMO_POINTS.punto3.id)!);
        expect(pointForNewAlert(store)).toBe(DEMO_POINTS.punto3.id);
    });
});

describe("alertToRaise", () => {
    it("elige la alerta activa más reciente que no está en Rojo", () => {
        const store = setup();
        expect(alertToRaise(store)).toBeNull();

        const first = store.openAlert("AMARILLO", DEMO_POINTS.punto2.id)!;
        const second = store.openAlert("ROJO", DEMO_POINTS.punto3.id)!;
        expect(alertToRaise(store)).toBe(first.id);

        store.finishAlert(second.id);
        const third = store.openAlert("NARANJA", DEMO_POINTS.punto3.id)!;
        expect(alertToRaise(store)).toBe(third.id);
    });
});

describe("alertToClassify", () => {
    it("elige la alerta activa más reciente sin clasificar", () => {
        const store = setup();
        expect(alertToClassify(store)).toBeNull();

        const first = store.openAlert("AMARILLO", DEMO_POINTS.punto2.id)!;
        const second = store.openAlert("ROJO", DEMO_POINTS.punto3.id)!;
        expect(alertToClassify(store)).toBe(second.id);

        store.classify(second.id, { classification: "CONFIRMADA", origin: "WEB" });
        expect(alertToClassify(store)).toBe(first.id);

        store.classify(first.id, { classification: "FALSA_ALERTA", origin: "WEB" });
        expect(alertToClassify(store)).toBeNull();
    });
});
