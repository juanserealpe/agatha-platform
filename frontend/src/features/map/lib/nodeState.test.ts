import { describe, expect, it } from "vitest";
import { getNodeState } from "./nodeState";

describe("getNodeState", () => {
    it("gris si el punto no tiene dispositivo enlazado", () => {
        expect(getNodeState({ isLinked: false, isOffline: false, alerts: null }).tone).toBe("unavailable");
    });

    it("normal si está enlazado, comunica y no tiene alertas", () => {
        expect(getNodeState({ isLinked: true, isOffline: false, alerts: null })).toEqual({
            tone: "normal",
            hasWarning: false,
            isBlinking: false,
        });
    });

    it("gris si dejó de comunicarse y no tiene alertas", () => {
        expect(getNodeState({ isLinked: true, isOffline: true, alerts: null }).tone).toBe("unavailable");
    });

    it("toma el nivel y el titileo que calcula el backend", () => {
        const state = getNodeState({ isLinked: true, isOffline: false, alerts: { level: "ROJO", blinking: true } });
        expect(state).toEqual({ tone: "ROJO", hasWarning: false, isBlinking: true });
    });

    it("alerta + aviso conserva el color de la alerta", () => {
        const state = getNodeState({ isLinked: true, isOffline: true, alerts: { level: "NARANJA", blinking: false } });
        expect(state).toMatchObject({ tone: "NARANJA", hasWarning: true });
    });
});
