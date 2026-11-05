import { describe, expect, it } from "vitest";
import { toAlertId, type AlertItem, type AlertSearchResponse } from "@/services/alertManagement";
import { pageCount, patchDetailManagement, replaceHistoryItem } from "./historyCache";

function alertItem(id: number, overrides: Partial<AlertItem> = {}): AlertItem {
    return {
        id: toAlertId(id),
        code: `ALR-${String(id).padStart(6, "0")}`,
        device: { id: 11556, pk: "PK34+500" },
        event: "FUGA",
        state: "ACTIVA",
        started_at: "2026-08-24T11:00:00-05:00",
        ended_at: null,
        in_progress: true,
        current_level: "ROJO",
        max_level: "ROJO",
        blinking: true,
        classification: null,
        management_status: "SIN_CLASIFICAR",
        label: null,
        label_other_text: null,
        available_actions: ["CONFIRM", "FALSE_ALERT", "VIEW"],
        ...overrides,
    };
}

const page: AlertSearchResponse = { total: 2, page: 1, page_size: 10, items: [alertItem(1), alertItem(2)] };

describe("replaceHistoryItem", () => {
    it("reemplaza la fila de la alerta y conserva las demás", () => {
        const updated = alertItem(2, { classification: "CONFIRMADA", management_status: "CLASIFICADA_SIN_ETIQUETA" });

        const result = replaceHistoryItem(page, updated);

        expect(result?.items[1]).toEqual(updated);
        expect(result?.items[0]).toBe(page.items[0]);
        expect(result?.total).toBe(2);
    });

    it("conserva el nombre del dispositivo de la fila (la respuesta trae MP-<id>)", () => {
        const updated = alertItem(1, { device: { id: 11556, pk: "MP-11556" }, blinking: false });

        expect(replaceHistoryItem(page, updated)?.items[0].device.pk).toBe("PK34+500");
    });

    it("no toca la página si la alerta no está en ella", () => {
        expect(replaceHistoryItem(page, alertItem(99))).toBe(page);
        expect(replaceHistoryItem(undefined, alertItem(1))).toBeUndefined();
    });
});

describe("pageCount", () => {
    it.each([
        [23, 10, 3],
        [20, 10, 2],
        [1, 10, 1],
        [0, 10, 1],
    ])("total %d con %d por página → %d", (total, size, expected) => {
        expect(pageCount(total, size)).toBe(expected);
    });
});

describe("patchDetailManagement", () => {
    it("actualiza solo los campos de gestión", () => {
        const detail = { ...alertItem(1), opened_at: "2026-08-24T11:00:00-05:00" };
        const updated = alertItem(1, {
            device: { id: 11556, pk: "MP-11556" },
            classification: "FALSA_ALERTA",
            management_status: "CLASIFICADA_SIN_ETIQUETA",
            available_actions: ["ADD_LABEL", "VIEW"],
            blinking: false,
        });

        const result = patchDetailManagement(detail, updated);

        expect(result).toMatchObject({
            classification: "FALSA_ALERTA",
            management_status: "CLASIFICADA_SIN_ETIQUETA",
            available_actions: ["ADD_LABEL", "VIEW"],
            blinking: false,
            opened_at: "2026-08-24T11:00:00-05:00",
        });
        expect(result?.device.pk).toBe("PK34+500");
    });

    it("no toca el detalle de otra alerta", () => {
        const detail = alertItem(1);
        expect(patchDetailManagement(detail, alertItem(2))).toBe(detail);
        expect(patchDetailManagement(undefined, alertItem(1))).toBeUndefined();
    });
});
