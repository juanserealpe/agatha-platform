import { describe, expect, it } from "vitest";
import { DEFAULT_LEAK_THRESHOLDS } from "../config/leakThresholds";
import { leakLevelOf, leakThresholdBands } from "./leakLevel";

describe("leakLevelOf", () => {
    it.each([
        [0, "normal"],
        [0.1799, "normal"],
        [0.16, "amarillo"],
        [0.2499, "amarillo"],
        [0.25, "naranja"],
        [0.35, "rojo"],
        [1, "rojo"],
    ])("I = %d → %s (cada umbral es inclusivo, como en el Rule Engine)", (value, expected) => {
        expect(leakLevelOf(value, DEFAULT_LEAK_THRESHOLDS)).toBe(expected);
    });

    it("usa los umbrales que recibe, no la plantilla", () => {
        expect(leakLevelOf(0.2, { amarillo: 0.1, naranja: 0.15, rojo: 0.19 })).toBe("rojo");
    });
});

describe("leakThresholdBands", () => {
    it("una franja por nivel, contiguas y la última abierta hasta el tope", () => {
        const bands = leakThresholdBands(DEFAULT_LEAK_THRESHOLDS);
        expect(bands.map((b) => [b.from, b.to])).toEqual([[0.16, 0.25], [0.25, 0.35], [0.35, undefined]]);
        expect(bands[0].label).toBe("Amarillo ≥ 0,16");
    });
});
