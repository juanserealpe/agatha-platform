import { describe, expect, it } from "vitest";
import { toAlertId, type AlertSearchFilters } from "@/services/alertManagement";
import { isApiError } from "@/services/http/apiError";
import { createDemoAlertStore, type DemoStreamEvent } from "./alertStore";
import { DEMO_POINTS, SCENARIO_POINT_ID } from "./demoData";

const NO_FILTERS: AlertSearchFilters = { device: null, from: null, to: null, maxLevels: [], events: [], statuses: [] };

function setup(start = "2026-10-31T15:20:00Z") {
    let current = new Date(start);
    const events: DemoStreamEvent[] = [];
    const store = createDemoAlertStore({ now: () => current });
    store.subscribe((event) => events.push(event));
    return {
        store,
        events,
        advance(minutes: number) {
            current = new Date(current.getTime() + minutes * 60_000);
        },
    };
}

function errorOf(action: () => unknown) {
    try {
        action();
    } catch (error) {
        if (isApiError(error)) return { status: error.status, code: error.code, message: error.message };
        throw error;
    }
    throw new Error("se esperaba un error");
}

describe("createDemoAlertStore", () => {
    it("abre ALR-900001 en el Punto 1 y emite la foto y el aviso", () => {
        const { store, events } = setup();
        const item = store.openAlert();

        expect(item).toMatchObject({
            id: 900001,
            code: "ALR-900001",
            device: { id: SCENARIO_POINT_ID, pk: DEMO_POINTS.punto1.name },
            state: "ACTIVA",
            current_level: "AMARILLO",
            blinking: false,
            management_status: "SIN_CLASIFICAR",
            available_actions: ["CONFIRM", "FALSE_ALERT", "VIEW"],
        });
        // Abre tras horas de ruido: el inicio queda antes, alineado a la hora (15:00Z - 9 h).
        expect(item?.started_at).toBe("2026-10-31T01:00:00-05:00");
        expect(events.map((event) => event.type)).toEqual(["changed", "notification"]);
        expect(events[0]).toMatchObject({ data: { change: "OPENED", evaluated_at: "2026-10-31T15:20:00Z" } });
        expect(events[1]).toMatchObject({
            data: {
                code: "OPEN",
                level: "AMARILLO",
                late: false,
                title: "Alerta nueva en Sensor MP-9001 PK34-470: nivel Amarillo",
                body: "Índice de ruido en 0,21 desde las 01:00 del 31/10; lo normal es menos de 0,16.",
            },
        });
    });

    it("no abre una segunda alerta en un punto con una activa", () => {
        const { store } = setup();
        store.openAlert();
        expect(store.openAlert()).toBeNull();
        expect(store.openAlert("AMARILLO", DEMO_POINTS.punto2.id)?.code).toBe("ALR-900002");
    });

    it("sube de nivel hasta Rojo, titila y no pasa de ahí", () => {
        const { store, events } = setup();
        const id = store.openAlert()!.id;
        store.raiseLevel(id);
        const red = store.raiseLevel(id);

        expect(red).toMatchObject({ current_level: "ROJO", max_level: "ROJO", blinking: true });
        expect(store.raiseLevel(id)).toBeNull();
        const notifications = events.filter((event) => event.type === "notification").map((event) => event.data);
        expect(notifications.map((notice) => notice.code)).toEqual(["OPEN", "UP_NARANJA_FAST", "UP_ROJO"]);
        expect(new Set(notifications.map((notice) => notice.message_id)).size).toBe(3);
        expect(notifications[2]).toMatchObject({
            title: "La alerta ALR-900001 subió a Rojo",
            body: "El índice de ruido pasó de 0,30 a 0,42; Rojo empieza en 0,35.",
        });
    });

    it("da a cada foto un evaluated_at mayor aunque los pasos ocurran en el mismo segundo", () => {
        const { store, events } = setup();
        const id = store.openAlert()!.id;
        store.raiseLevel(id);
        store.raiseLevel(id);

        const evaluated = events.filter((event) => event.type === "changed").map((event) => Date.parse(event.data.evaluated_at));
        expect(evaluated).toEqual([...evaluated].sort((a, b) => a - b));
        expect(new Set(evaluated).size).toBe(evaluated.length);
    });

    it("arma la línea de tiempo por nivel con tramos contiguos", () => {
        const { store, advance } = setup();
        const id = store.openAlert()!.id;
        advance(1);
        store.raiseLevel(id);
        advance(1);
        store.raiseLevel(id);
        advance(1);
        store.finishAlert(id);

        const detail = store.getDetail(id);
        expect(detail.levels.map((segment) => [segment.level, segment.opened_by])).toEqual([
            ["AMARILLO", "OPEN"],
            ["NARANJA", "UP"],
            ["ROJO", "UP"],
        ]);
        expect(detail.levels[0].ended_at).toBe(detail.levels[1].started_at);
        expect(detail.levels[1].ended_at).toBe(detail.levels[2].started_at);
        expect(detail.levels[2].messages.map((message) => message.code)).toEqual(["UP_ROJO", "END_NORMAL"]);
        expect(detail).toMatchObject({ state: "FINALIZADA", current_level: null, in_progress: false, last_indicator_value: 0.08 });
        expect(detail.hours_calculated).toBe(9);
    });

    it("al terminar emite FINISHED sin aviso y sale del mapa, pero sigue pendiente", () => {
        const { store, events } = setup();
        const id = store.openAlert()!.id;
        events.length = 0;
        store.finishAlert(id);

        expect(events).toHaveLength(1);
        expect(events[0]).toMatchObject({ type: "changed", data: { change: "FINISHED", state: "FINALIZADA", current_level: null } });
        expect(store.getActive().devices).toEqual([]);
        expect(store.getPending().total).toBe(1);
        expect(store.finishAlert(id)).toBeNull();
    });

    it("expone los tramos de nivel del punto para las series", () => {
        const { store } = setup();
        const id = store.openAlert()!.id;
        store.raiseLevel(id);

        const intervals = store.levelIntervals(SCENARIO_POINT_ID);
        expect(intervals.map((interval) => interval.level)).toEqual(["AMARILLO", "NARANJA"]);
        expect(intervals[0].to).toEqual(intervals[1].from);
        expect(intervals[1].to).toBeNull();
        expect(store.levelIntervals(DEMO_POINTS.punto2.id)).toEqual([]);
    });

    it("una alerta nueva en el mismo punto no se superpone con la anterior", () => {
        const { store, advance } = setup();
        const first = store.openAlert()!.id;
        advance(1);
        store.finishAlert(first);
        advance(1);
        store.openAlert();

        const intervals = store.levelIntervals(SCENARIO_POINT_ID);
        expect(intervals[1].from.getTime()).toBeGreaterThanOrEqual(intervals[0].to!.getTime());
    });

    describe("consultas", () => {
        it("agrupa las activas por punto con el nivel más alto", () => {
            const { store } = setup();
            store.openAlert("ROJO", DEMO_POINTS.punto3.id);
            store.openAlert("AMARILLO", DEMO_POINTS.punto1.id);

            const { devices } = store.getActive();
            expect(devices.map((device) => [device.measuring_point_id, device.level, device.blinking])).toEqual([
                [DEMO_POINTS.punto1.id, "AMARILLO", false],
                [DEMO_POINTS.punto3.id, "ROJO", true],
            ]);
        });

        it("filtra y pagina de la más reciente a la más antigua", () => {
            const { store, advance } = setup();
            for (const point of Object.values(DEMO_POINTS)) {
                store.openAlert("AMARILLO", point.id);
                advance(120);
            }

            expect(store.search(NO_FILTERS, 1).items.map((item) => item.code)).toEqual(["ALR-900003", "ALR-900002", "ALR-900001"]);
            expect(store.search({ ...NO_FILTERS, device: DEMO_POINTS.punto2.id }, 1).total).toBe(1);
            expect(store.search({ ...NO_FILTERS, maxLevels: ["ROJO"] }, 1).total).toBe(0);
            expect(store.search({ ...NO_FILTERS, from: "2026-11-01" }, 1).total).toBe(0);
            expect(store.search(NO_FILTERS, 2).items).toEqual([]);
        });

        it("responde 404 con una alerta que no existe", () => {
            const { store } = setup();
            expect(errorOf(() => store.getDetail(toAlertId(123)))).toMatchObject({ status: 404, code: "NOT_FOUND" });
        });
    });

    describe("gestión", () => {
        it("clasifica como el backend: deja de titilar, sale de pendientes y emite CLASSIFIED", () => {
            const { store, events } = setup();
            const id = store.openAlert("ROJO")!.id;
            events.length = 0;

            const result = store.classify(id, { classification: "CONFIRMADA", origin: "WEB", observation: "Humedad en la abscisa" });

            expect(result).toMatchObject({
                outcome: "SAVED",
                alert: {
                    classification: "CONFIRMADA",
                    management_status: "CLASIFICADA_SIN_ETIQUETA",
                    blinking: false,
                    available_actions: ["ADD_LABEL", "VIEW"],
                },
            });
            expect(events).toEqual([expect.objectContaining({ type: "changed", data: expect.objectContaining({ change: "CLASSIFIED" }) })]);
            expect(store.getPending().total).toBe(0);
            const detail = store.getDetail(id);
            expect(detail.observations).toMatchObject([{ text: "Humedad en la abscisa", kind: "OBSERVACION", step: "CLASIFICACION" }]);
            expect(detail.levels[0].messages.at(-1)).toMatchObject({
                kind: "GESTION",
                title: "Clasificada como Confirmada",
                body: "Registrada desde Web.",
            });
        });

        it("una segunda clasificación queda como observación de conflicto y prevalece la primera", () => {
            const { store, events } = setup();
            const id = store.openAlert()!.id;
            store.classify(id, { classification: "CONFIRMADA", origin: "WEB" });
            events.length = 0;

            const result = store.classify(id, { classification: "FALSA_ALERTA", origin: "APP_MOVIL", device_identifier: "cel-1" });

            expect(result).toMatchObject({
                outcome: "SAVED_AS_CONFLICT_OBSERVATION",
                alert: { classification: "CONFIRMADA" },
                existing: { classification: "CONFIRMADA", origin: "WEB" },
            });
            expect(store.getDetail(id).observations.at(-1)).toMatchObject({
                text: "Clasificación en conflicto: Falsa alerta desde App móvil (cel-1)",
                kind: "CONFLICTO_CLASIFICACION",
            });
            expect(events).toEqual([]);
        });

        it("reenviar el mismo client_record_id no tiene efecto", () => {
            const { store } = setup();
            const id = store.openAlert()!.id;
            const request = { classification: "CONFIRMADA", origin: "WEB", client_record_id: "a1" } as const;
            store.classify(id, request);

            expect(store.classify(id, request).outcome).toBe("ALREADY_RECEIVED");
            expect(store.getDetail(id).observations).toEqual([]);
        });

        it("valida la clasificación, el origen y la observación", () => {
            const { store } = setup();
            const id = store.openAlert()!.id;

            expect(errorOf(() => store.classify(id, { classification: "OTRA" as never, origin: "WEB" }))).toMatchObject({ status: 400 });
            expect(errorOf(() => store.classify(id, { classification: "CONFIRMADA", origin: "APP_MOVIL" }))).toMatchObject({ status: 400 });
            expect(
                errorOf(() => store.classify(id, { classification: "CONFIRMADA", origin: "WEB", observation: "x".repeat(501) }))
            ).toMatchObject({ message: "la observación debe tener entre 1 y 500 caracteres" });
        });

        it("etiqueta solo después de clasificar y con la lista de su clasificación", () => {
            const { store, events } = setup();
            const id = store.openAlert()!.id;

            expect(errorOf(() => store.label(id, { label: "MOVIMIENTO_TIERRA", origin: "WEB" }))).toMatchObject({ status: 409, code: "CONFLICT" });
            store.classify(id, { classification: "CONFIRMADA", origin: "WEB" });
            expect(errorOf(() => store.label(id, { label: "CLIMA_EXTREMO", origin: "WEB" }))).toMatchObject({ status: 400 });
            expect(errorOf(() => store.label(id, { label: "OTRO", origin: "WEB" }))).toMatchObject({ status: 400 });
            expect(errorOf(() => store.label(id, { label: "MOVIMIENTO_TIERRA", origin: "WEB", other_text: "x" }))).toMatchObject({ status: 400 });

            events.length = 0;
            const result = store.label(id, { label: "OTRO", other_text: " Raíz de árbol ", origin: "WEB" });

            expect(result).toMatchObject({
                outcome: "SAVED",
                alert: { label: "OTRO", label_other_text: "Raíz de árbol", management_status: "CLASIFICADA_CON_ETIQUETA", available_actions: ["VIEW"] },
            });
            expect(events[0]).toMatchObject({ data: { change: "LABELED" } });
            expect(store.getDetail(id).levels[0].messages.at(-1)).toMatchObject({ title: "Etiqueta: Otro: Raíz de árbol" });
            expect(store.label(id, { label: "MAQUINARIA_AJENA", origin: "WEB" })).toMatchObject({
                outcome: "SAVED_AS_CONFLICT_OBSERVATION",
                existing: { label: "OTRO", label_other_text: "Raíz de árbol" },
            });
        });

        it("agrega observaciones libres", () => {
            const { store } = setup();
            const id = store.openAlert()!.id;

            expect(store.addObservation(id, { text: "Visita mañana", origin: "WEB" }).outcome).toBe("SAVED");
            expect(store.getDetail(id).observations).toMatchObject([{ text: "Visita mañana", step: "LIBRE" }]);
            expect(errorOf(() => store.addObservation(id, { text: "  ", origin: "WEB" }))).toMatchObject({ status: 400 });
        });
    });

    it("reset borra todo y vuelve a empezar en ALR-900001", () => {
        const { store } = setup();
        store.openAlert();
        store.reset();

        expect(store.search(NO_FILTERS, 1).total).toBe(0);
        expect(store.openAlert()?.code).toBe("ALR-900001");
    });

    it("deja de avisar a quien se desuscribe", () => {
        const store = createDemoAlertStore();
        const received: DemoStreamEvent[] = [];
        const unsubscribe = store.subscribe((event) => received.push(event));
        unsubscribe();
        store.openAlert();

        expect(received).toEqual([]);
    });
});
