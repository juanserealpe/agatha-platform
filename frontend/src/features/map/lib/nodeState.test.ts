import { describe, expect, it } from "vitest";
import type { Alert } from "@/services/alerts";
import { getNodeState, pickMostSevereAlert } from "./nodeState";

const alert = (overrides: Partial<Alert>): Alert => ({
    id: "a",
    severity: "yellow",
    type: "leak",
    status: "unclassified",
    deviceId: 1,
    pkMeters: 34470,
    detectedAt: "2026-10-18T12:00:00Z",
    ...overrides,
});

describe("getNodeState", () => {
    it("gris si el punto no tiene dispositivo enlazado", () => {
        expect(getNodeState({ isLinked: false, isOffline: false, alerts: [] }).tone).toBe("unavailable");
    });

    it("normal si está enlazado, comunica y no tiene alertas", () => {
        expect(getNodeState({ isLinked: true, isOffline: false, alerts: [] })).toEqual({
            tone: "normal",
            hasWarning: false,
            isBlinking: false,
        });
    });

    it("gris si dejó de comunicarse y no tiene alertas", () => {
        expect(getNodeState({ isLinked: true, isOffline: true, alerts: [] }).tone).toBe("unavailable");
    });

    it("toma la alerta más grave", () => {
        const alerts = [alert({ severity: "yellow" }), alert({ severity: "orange" })];
        expect(getNodeState({ isLinked: true, isOffline: false, alerts }).tone).toBe("orange");
    });

    it("la roja titila solo mientras está sin clasificar", () => {
        const pending = getNodeState({ isLinked: true, isOffline: false, alerts: [alert({ severity: "red" })] });
        const confirmed = getNodeState({
            isLinked: true,
            isOffline: false,
            alerts: [alert({ severity: "red", status: "confirmed" })],
        });
        expect(pending.isBlinking).toBe(true);
        expect(confirmed.isBlinking).toBe(false);
    });

    it("alerta + aviso conserva el color de la alerta", () => {
        const state = getNodeState({ isLinked: true, isOffline: true, alerts: [alert({ severity: "orange" })] });
        expect(state).toMatchObject({ tone: "orange", hasWarning: true });
    });

    it("ignora las falsas alertas", () => {
        const alerts = [alert({ severity: "red", status: "dismissed" })];
        expect(getNodeState({ isLinked: true, isOffline: false, alerts }).tone).toBe("normal");
    });
});

describe("pickMostSevereAlert", () => {
    it("devuelve la más grave e ignora las falsas alertas", () => {
        const red = alert({ id: "r", severity: "red", status: "dismissed" });
        const orange = alert({ id: "o", severity: "orange" });
        expect(pickMostSevereAlert([red, orange])?.id).toBe("o");
        expect(pickMostSevereAlert([])).toBeNull();
    });
});
