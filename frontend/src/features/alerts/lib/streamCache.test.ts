import { describe, expect, it } from "vitest";
import { toAlertId, type ActiveResponse, type AlertItem, type PendingResponse } from "@/services/alertManagement";
import type { AlertChangedEvent } from "@/services/realtime";
import { applyToActive, applyToPending, isNewerEvaluation, mergeAlertItem } from "./streamCache";

const item = (id: number, overrides: Partial<AlertItem> = {}): AlertItem => ({
    id: toAlertId(id),
    code: `ALR-${String(id).padStart(6, "0")}`,
    device: { id: 11556, pk: "Sensor MP-11556 PK34-470" },
    event: "FUGA",
    state: "ACTIVA",
    started_at: "2026-10-30T07:00:00-05:00",
    ended_at: null,
    in_progress: true,
    current_level: "AMARILLO",
    max_level: "AMARILLO",
    blinking: false,
    classification: null,
    management_status: "SIN_CLASIFICAR",
    label: null,
    label_other_text: null,
    available_actions: ["CONFIRM", "FALSE_ALERT", "VIEW"],
    ...overrides,
});

const changed = (id: number, overrides: Partial<AlertChangedEvent> = {}): AlertChangedEvent => ({
    alert_id: toAlertId(id),
    alert_code: `ALR-${String(id).padStart(6, "0")}`,
    measuring_point_id: 11556,
    event_type: "FUGA",
    change: "LEVEL_RAISED",
    state: "ACTIVA",
    current_level: "ROJO",
    max_level: "ROJO",
    blinking: true,
    management_status: "SIN_CLASIFICAR",
    classification: null,
    started_at: "2026-10-30T12:00:00Z",
    evaluated_at: "2026-10-30T15:00:00Z",
    occurred_at: "2026-10-30T15:00:05Z",
    ...overrides,
});

const point = (alerts: AlertItem[]) => ({
    measuring_point_id: 11556,
    pk: "Sensor MP-11556 PK34-470",
    level: alerts[0].current_level!,
    blinking: alerts.some((alert) => alert.blinking),
    alerts,
});

describe("isNewerEvaluation", () => {
    it("aplica la primera foto y solo las posteriores", () => {
        expect(isNewerEvaluation("2026-10-30T15:00:00Z", undefined)).toBe(true);
        expect(isNewerEvaluation("2026-10-30T16:00:00Z", "2026-10-30T15:00:00Z")).toBe(true);
        expect(isNewerEvaluation("2026-10-30T15:00:00Z", "2026-10-30T15:00:00Z")).toBe(false);
    });
});

describe("mergeAlertItem", () => {
    it("aplica nivel y estado y recalcula el titileo", () => {
        const merged = mergeAlertItem(item(1), changed(1), true);
        expect(merged).toMatchObject({ current_level: "ROJO", max_level: "ROJO", blinking: true });
    });

    it("una foto vieja no cambia el nivel, pero la gestión sí avanza", () => {
        const red = item(1, { current_level: "ROJO", max_level: "ROJO", blinking: true });
        const old = changed(1, {
            current_level: "NARANJA",
            management_status: "CLASIFICADA_SIN_ETIQUETA",
            classification: "CONFIRMADA",
        });
        const merged = mergeAlertItem(red, old, false);
        expect(merged).toMatchObject({ current_level: "ROJO", classification: "CONFIRMADA", blinking: false });
    });

    it("la gestión nunca retrocede", () => {
        const labeled = item(1, { management_status: "CLASIFICADA_CON_ETIQUETA", classification: "FALSA_ALERTA" });
        const merged = mergeAlertItem(labeled, changed(1, { change: "LEVEL_RAISED" }), true);
        expect(merged.management_status).toBe("CLASIFICADA_CON_ETIQUETA");
        expect(merged.blinking).toBe(false);
    });
});

describe("applyToActive", () => {
    it("actualiza el punto con el nivel mayor y el titileo", () => {
        const data: ActiveResponse = { devices: [point([item(1), item(2)])] };
        const { data: next, refetch } = applyToActive(data, changed(1), true);
        expect(refetch).toBe(false);
        expect(next!.devices[0]).toMatchObject({ level: "ROJO", blinking: true });
    });

    it("quita la alerta finalizada y el punto si ya no tiene ninguna", () => {
        const data: ActiveResponse = { devices: [point([item(1)])] };
        const finished = changed(1, { change: "FINISHED", state: "FINALIZADA", current_level: null });
        expect(applyToActive(data, finished, true).data!.devices).toEqual([]);
    });

    it("pide reconsultar si llega una alerta activa que no está en caché", () => {
        const data: ActiveResponse = { devices: [] };
        expect(applyToActive(data, changed(9, { change: "OPENED" }), true)).toEqual({ data, refetch: true });
        expect(applyToActive(data, changed(9, { state: "FINALIZADA" }), true).refetch).toBe(false);
    });

    it("sin caché no hace nada", () => {
        expect(applyToActive(undefined, changed(1), true)).toEqual({ data: undefined, refetch: false });
    });
});

describe("applyToPending", () => {
    it("actualiza la fila que sigue sin clasificar, aunque haya finalizado", () => {
        const data: PendingResponse = { total: 1, items: [item(1)] };
        const finished = changed(1, { change: "FINISHED", state: "FINALIZADA", current_level: null });
        const { data: next } = applyToPending(data, finished, true);
        expect(next!.items[0]).toMatchObject({ state: "FINALIZADA", in_progress: false });
        expect(next!.total).toBe(1);
    });

    it("quita la alerta clasificada y reconsulta si quedaban más pendientes que filas", () => {
        const data: PendingResponse = { total: 7, items: [item(1), item(2)] };
        const classified = changed(1, {
            change: "CLASSIFIED",
            management_status: "CLASIFICADA_SIN_ETIQUETA",
            classification: "CONFIRMADA",
        });
        const { data: next, refetch } = applyToPending(data, classified, false);
        expect(next).toEqual({ total: 6, items: [item(2)] });
        expect(refetch).toBe(true);
    });

    it("una alerta nueva pide reconsultar; los cambios de una que no se ve, no", () => {
        const data: PendingResponse = { total: 0, items: [] };
        expect(applyToPending(data, changed(9, { change: "OPENED" }), true).refetch).toBe(true);
        expect(applyToPending(data, changed(9, { change: "LEVEL_RAISED" }), true).refetch).toBe(false);
    });
});
