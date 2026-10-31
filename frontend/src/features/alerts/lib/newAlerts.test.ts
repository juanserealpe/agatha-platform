import { describe, expect, it } from "vitest";
import { toAlertId, type AlertItem } from "@/services/alertManagement";
import { dismissAlert, ingestAlerts, INITIAL_NEW_ALERTS_STATE } from "./newAlerts";

// Solo importa el id: el resto de la fila no interviene en la detección de nuevas.
const alert = (id: number) => ({ id: toAlertId(id) }) as AlertItem;
const ids = (alerts: AlertItem[]) => alerts.map((a) => a.id);

describe("ingestAlerts", () => {
    it("la primera lista fija la línea base y no notifica", () => {
        const state = ingestAlerts(INITIAL_NEW_ALERTS_STATE, [alert(1), alert(2)]);
        expect(state.unseen).toEqual([]);
        expect([...state.seenIds!]).toEqual([1, 2]);
    });

    it("una lista vacía también fija la línea base", () => {
        const state = ingestAlerts(INITIAL_NEW_ALERTS_STATE, []);
        expect(ids(ingestAlerts(state, [alert(1)]).unseen)).toEqual([1]);
    });

    it("notifica solo los ids que no había visto", () => {
        const base = ingestAlerts(INITIAL_NEW_ALERTS_STATE, [alert(1)]);
        const next = ingestAlerts(base, [alert(2), alert(1)]);
        expect(ids(next.unseen)).toEqual([2]);
    });

    it("no repite una alerta ya notificada aunque siga en la lista", () => {
        const base = ingestAlerts(INITIAL_NEW_ALERTS_STATE, []);
        const once = ingestAlerts(base, [alert(1)]);
        const twice = ingestAlerts(once, [alert(1)]);
        expect(twice).toBe(once);
    });

    it("una alerta descartada no vuelve a aparecer", () => {
        const base = ingestAlerts(INITIAL_NEW_ALERTS_STATE, []);
        const dismissed = dismissAlert(ingestAlerts(base, [alert(1)]), toAlertId(1));
        expect(ingestAlerts(dismissed, [alert(1)]).unseen).toEqual([]);
    });
});
