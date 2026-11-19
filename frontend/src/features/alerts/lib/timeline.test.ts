import { describe, expect, it } from "vitest";
import { ANOMALY_PHRASE, EVENT_EXPLANATION, formatAnomalyScore, leakSignatureText } from "../config/alertExplanations";
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
    it("cada evento tiene título y una frase por nivel", () => {
        expect(EVENT_EXPLANATION.FUGA.title).toBe("Anomalía acústica");
        expect(EVENT_EXPLANATION.MOVIMIENTO.title).toBe("Posible movimiento del terreno");
        for (const phrases of Object.values(ANOMALY_PHRASE)) {
            expect(Object.keys(phrases)).toEqual(["AMARILLO", "NARANJA", "ROJO"]);
        }
    });

    it("ningún texto presenta el indicador como probabilidad o porcentaje", () => {
        const texts = [...Object.values(EVENT_EXPLANATION).map((e) => e.description), ...Object.values(ANOMALY_PHRASE).flatMap(Object.values)];
        texts.forEach((text) => expect(text).not.toMatch(/%|probabilidad/i));
    });

    it("firma de fuga: confirmada, no confirmada y sin dato", () => {
        expect(leakSignatureText({ d_pl: 12.34, d_pl_min: 10, confirmed: true })).toEqual({
            title: "Con firma de fuga",
            text: "El 12,3 % de los registros de las últimas 24 h tiene firma de fuga (mínimo 10 %): el sonido se parece al de una fuga.",
        });
        expect(leakSignatureText({ d_pl: 4.6, d_pl_min: 10, confirmed: false })?.title).toBe("Sin firma de fuga");
        expect(leakSignatureText({ d_pl: null, d_pl_min: 10, confirmed: false })).toBeNull();
        expect(leakSignatureText(null)).toBeNull();
        expect(leakSignatureText(undefined)).toBeNull();
    });

    it("el puntaje se muestra como número, no como porcentaje", () => {
        expect(formatAnomalyScore(3.256)).toBe("3,26");
        expect(formatAnomalyScore(Number.NaN)).toBe("—");
    });
});
