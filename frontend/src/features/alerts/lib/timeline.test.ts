import { describe, expect, it } from "vitest";
import { EVENT_EXPLANATION, formatAnomalyScore } from "../config/alertExplanations";
import { formatDuration, segmentDurationMs, totalDurationMs } from "./timeline";

const HOUR = 3_600_000;
const NOW = new Date("2026-08-24T18:00:00-05:00").getTime();

describe("segmentDurationMs", () => {
    it("tramo cerrado", () => {
        expect(segmentDurationMs({ started_at: "2026-08-24T14:30:00-05:00", ended_at: "2026-08-24T16:15:00-05:00" }, NOW)).toBe(
            1.75 * HOUR
        );
    });

    it("tramo abierto cuenta hasta ahora", () => {
        expect(segmentDurationMs({ started_at: "2026-08-24T16:15:00-05:00", ended_at: null }, NOW)).toBe(1.75 * HOUR);
    });

    it("nunca negativo", () => {
        expect(segmentDurationMs({ started_at: "2026-08-24T19:00:00-05:00", ended_at: null }, NOW)).toBe(0);
    });
});

describe("totalDurationMs", () => {
    it("suma los tramos", () => {
        expect(
            totalDurationMs(
                [
                    { started_at: "2026-08-24T14:30:00-05:00", ended_at: "2026-08-24T16:15:00-05:00" },
                    { started_at: "2026-08-24T16:15:00-05:00", ended_at: null },
                ],
                NOW
            )
        ).toBe(3.5 * HOUR);
    });

    it("sin tramos es cero", () => {
        expect(totalDurationMs([], NOW)).toBe(0);
    });
});

describe("formatDuration", () => {
    it.each([
        [30_000, "< 1 min"],
        [45 * 60_000, "45 min"],
        [3 * HOUR, "3 h"],
        [3.5 * HOUR, "3 h 30 min"],
        [38 * HOUR, "38 h"],
        [72 * HOUR, "3 d"],
        [80 * HOUR, "3 d 8 h"],
    ])("%d ms → %s", (ms, expected) => {
        expect(formatDuration(ms)).toBe(expected);
    });
});

describe("textos en lenguaje claro", () => {
    it("fuga tiene frase para los tres niveles y puntaje de anomalía", () => {
        expect(EVENT_EXPLANATION.FUGA.title).toBe("Anomalía acústica");
        expect(Object.keys(EVENT_EXPLANATION.FUGA.levelPhrase)).toEqual(["AMARILLO", "NARANJA", "ROJO"]);
        expect(EVENT_EXPLANATION.FUGA.hasAnomalyScore).toBe(true);
    });

    it("movimiento habla del sensor, tiene los tres niveles (motion-v2) y no muestra puntaje (A7)", () => {
        const motion = EVENT_EXPLANATION.MOVIMIENTO;
        expect(motion.title).toBe("Cambio de inclinación del sensor");
        expect(Object.keys(motion.levelPhrase)).toEqual(["AMARILLO", "NARANJA", "ROJO"]);
        expect(motion.hasAnomalyScore).toBe(false);
        [motion.title, motion.description, ...Object.values(motion.levelPhrase)].forEach((text) =>
            expect(text).not.toMatch(/cierre|cerr|reapertura|flujo de gas/i),
        );
        [motion.title, ...Object.values(motion.levelPhrase)].forEach((text) => expect(text).not.toMatch(/terreno/i));
    });

    it("movimiento nombra el terreno solo como posibilidad a verificar en campo", () => {
        const { description } = EVENT_EXPLANATION.MOVIMIENTO;
        expect(description).toMatch(/deslizamiento/);
        expect(description).toMatch(/puede deberse a un movimiento del terreno/i);
        expect(description).toMatch(/verificación en campo/);
    });

    it("ningún texto presenta el indicador como probabilidad o porcentaje", () => {
        const texts = Object.values(EVENT_EXPLANATION).flatMap((e) => [e.description, ...Object.values(e.levelPhrase)]);
        texts.forEach((text) => expect(text).not.toMatch(/%|probabilidad/i));
    });

    it("el puntaje se muestra como número, no como porcentaje", () => {
        expect(formatAnomalyScore(3.256)).toBe("3,26");
        expect(formatAnomalyScore(Number.NaN)).toBe("—");
    });
});
