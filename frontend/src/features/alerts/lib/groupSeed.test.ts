import { describe, expect, it } from "vitest";
import { toAlertId, type AlertItem } from "@/services/alertManagement";
import { findListedAlert, groupSeedFromItem, type ListedAlerts } from "./groupSeed";

function item(id: number, overrides: Partial<AlertItem> = {}): AlertItem {
    return {
        id: toAlertId(id),
        code: `ALR-${id}`,
        device: { id: 11556, pk: "Sensor PK34-470" },
        event: "FUGA",
        state: "FINALIZADA",
        started_at: "2026-11-20T10:00:00-05:00",
        ended_at: "2026-11-20T14:00:00-05:00",
        in_progress: false,
        current_level: null,
        max_level: "NARANJA",
        blinking: false,
        classification: null,
        management_status: "SIN_CLASIFICAR",
        label: null,
        label_other_text: null,
        available_actions: [],
        ...overrides,
    } as AlertItem;
}

const EMPTY: ListedAlerts = { histories: [], pending: undefined, active: undefined };

describe("findListedAlert", () => {
    it("encuentra la alerta en Sin clasificar, en el mapa o en una página del historial", () => {
        const lists: ListedAlerts = {
            pending: { total: 1, items: [item(1)] },
            active: { devices: [{ measuring_point_id: 11556, pk: "PK", level: "ROJO", blinking: false, alerts: [item(2)] }] },
            histories: [undefined, { total: 1, page: 1, page_size: 20, items: [item(3)] }],
        };
        expect(findListedAlert(lists, toAlertId(1))?.id).toBe(1);
        expect(findListedAlert(lists, toAlertId(2))?.id).toBe(2);
        expect(findListedAlert(lists, toAlertId(3))?.id).toBe(3);
    });

    it("devuelve undefined si no está en ninguna lista (p. ej. un enlace directo)", () => {
        expect(findListedAlert(EMPTY, toAlertId(9))).toBeUndefined();
    });
});

describe("groupSeedFromItem", () => {
    it("conserva el rango y el estado de la fila, que es lo que usa la búsqueda del grupo", () => {
        const row = item(7, { state: "ACTIVA", ended_at: null });
        expect(groupSeedFromItem(row)).toMatchObject({
            id: 7,
            state: "ACTIVA",
            started_at: row.started_at,
            ended_at: null,
            levels: [],
        });
    });
});
