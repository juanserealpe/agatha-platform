import { describe, expect, it } from "vitest";
import { cleanUnit, formatNumber, formatWithUnit, makeTickFormatter, stepDecimals } from "./formatValue";

describe("formatNumber", () => {
    it("decimales según la magnitud", () => {
        expect([formatNumber(947), formatNumber(22.094), formatNumber(0.03772)]).toEqual(["947", "22,09", "0,038"]);
    });
});

describe("formatWithUnit", () => {
    it("pega % y ° al número y separa el resto", () => {
        expect(formatWithUnit(5.26, "%")).toBe("5,26%");
        expect(formatWithUnit(-3.17, "°")).toBe("-3,17°");
        expect(formatWithUnit(0.975, "g")).toBe("0,975 g");
    });

    it('sin unidad cuando el backend manda "-"', () => {
        expect(cleanUnit("-")).toBe("");
        expect(formatWithUnit(53, "-")).toBe("53");
    });
});

describe("marcas del eje", () => {
    it.each([[100, 0], [2.5, 1], [0.25, 2], [0.1, 1], [0.0005, 4]])("paso %d → %d decimales", (step, digits) => {
        expect(stepDecimals(step)).toBe(digits);
    });

    it("marcas vecinas nunca se leen igual", () => {
        const fmt = makeTickFormatter(0.0005);
        expect(fmt(0.9745)).not.toBe(fmt(0.975));
        expect(makeTickFormatter(10)(20)).toBe("20");
    });
});
