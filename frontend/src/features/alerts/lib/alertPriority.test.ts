import { describe, expect, it } from "vitest";
import { toAlertId, type AlertItem } from "@/services/alertManagement";
import { activeAlertsByPriority, mostUrgentAlert } from "./alertPriority";

const item = (id: number, overrides: Partial<AlertItem> = {}): AlertItem => ({
    id: toAlertId(id),
    code: `ALR-${String(id).padStart(6, "0")}`,
    device: { id: 11556, pk: "Sensor MP-11556 PK34-470" },
    event: "FUGA",
    state: "ACTIVA",
    started_at: "2026-10-18T07:00:00-05:00",
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

describe("mostUrgentAlert", () => {
    it("prioriza el mayor nivel aunque sea más antigua", () => {
        const yellow = item(1, { started_at: "2026-10-18T09:00:00-05:00" });
        const orange = item(2, { current_level: "NARANJA", max_level: "NARANJA" });
        expect(mostUrgentAlert([yellow, orange])?.id).toBe(2);
    });

    it("entre el mismo nivel, la que titila y después la más reciente", () => {
        const red = { current_level: "ROJO", max_level: "ROJO" } as const;
        const classified = item(1, { ...red, started_at: "2026-10-18T09:00:00-05:00" });
        const blinkingOld = item(2, { ...red, blinking: true });
        const blinkingNew = item(3, { ...red, blinking: true, started_at: "2026-10-18T08:00:00-05:00" });
        expect(mostUrgentAlert([classified, blinkingOld, blinkingNew])?.id).toBe(3);
    });

    it("una finalizada cuenta con su nivel máximo", () => {
        const ended = item(1, { state: "FINALIZADA", current_level: null, max_level: "ROJO" });
        expect(mostUrgentAlert([item(2), ended])?.id).toBe(1);
    });

    it("null si no hay alertas", () => {
        expect(mostUrgentAlert([])).toBeNull();
    });
});

describe("activeAlertsByPriority", () => {
    it("junta las alertas de todos los puntos y las ordena por urgencia, clasificadas o no", () => {
        const confirmedRed = item(1, {
            current_level: "ROJO",
            max_level: "ROJO",
            management_status: "CLASIFICADA_SIN_ETIQUETA",
            classification: "CONFIRMADA",
        });
        const yellow = item(2);
        const orange = item(3, { current_level: "NARANJA", max_level: "NARANJA" });
        const point = (id: number, alerts: AlertItem[]) => ({
            measuring_point_id: id,
            pk: `MP-${id}`,
            level: alerts[0].current_level!,
            blinking: false,
            alerts,
        });

        const sorted = activeAlertsByPriority({ devices: [point(1, [yellow, orange]), point(2, [confirmedRed])] });

        expect(sorted.map((alert) => alert.id)).toEqual([1, 3, 2]);
        expect(activeAlertsByPriority(undefined)).toEqual([]);
    });
});
