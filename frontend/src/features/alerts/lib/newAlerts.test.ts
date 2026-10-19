import { describe, expect, it } from "vitest";
import type { Alert } from "@/services/alerts";
import { dismissAlert, ingestAlerts, INITIAL_NEW_ALERTS_STATE, pickHeadline } from "./newAlerts";

const alert = (id: string, overrides: Partial<Alert> = {}): Alert => ({
    id,
    severity: "yellow",
    type: "leak",
    status: "unclassified",
    deviceId: 11556,
    pkMeters: 34470,
    detectedAt: "2026-10-18T12:00:00Z",
    ...overrides,
});

describe("ingestAlerts", () => {
    it("la primera lista fija la línea base y no notifica", () => {
        const state = ingestAlerts(INITIAL_NEW_ALERTS_STATE, [alert("a"), alert("b")]);
        expect(state.unseen).toEqual([]);
        expect([...state.seenIds!]).toEqual(["a", "b"]);
    });

    it("una lista vacía también fija la línea base", () => {
        const state = ingestAlerts(INITIAL_NEW_ALERTS_STATE, []);
        expect(ingestAlerts(state, [alert("a")]).unseen.map((a) => a.id)).toEqual(["a"]);
    });

    it("notifica solo los ids que no había visto", () => {
        const base = ingestAlerts(INITIAL_NEW_ALERTS_STATE, [alert("a")]);
        const next = ingestAlerts(base, [alert("b"), alert("a")]);
        expect(next.unseen.map((a) => a.id)).toEqual(["b"]);
    });

    it("no repite una alerta ya notificada aunque siga en la lista", () => {
        const base = ingestAlerts(INITIAL_NEW_ALERTS_STATE, []);
        const once = ingestAlerts(base, [alert("a")]);
        const twice = ingestAlerts(once, [alert("a")]);
        expect(twice).toBe(once);
    });

    it("una alerta descartada no vuelve a aparecer", () => {
        const base = ingestAlerts(INITIAL_NEW_ALERTS_STATE, []);
        const dismissed = dismissAlert(ingestAlerts(base, [alert("a")]), "a");
        expect(ingestAlerts(dismissed, [alert("a")]).unseen).toEqual([]);
    });
});

describe("pickHeadline", () => {
    it("prioriza la más grave y, entre iguales, la más reciente", () => {
        const older = alert("old", { severity: "red", detectedAt: "2026-10-18T10:00:00Z" });
        const newer = alert("new", { severity: "red", detectedAt: "2026-10-18T11:00:00Z" });
        const yellow = alert("y", { detectedAt: "2026-10-18T12:00:00Z" });
        expect(pickHeadline([yellow, older, newer])?.id).toBe("new");
        expect(pickHeadline([])).toBeNull();
    });
});
