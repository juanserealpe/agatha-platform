import { describe, expect, it } from "vitest";
import { getBatteryVisual } from "./battery";

describe("getBatteryVisual", () => {
    it("verde desde 80 %", () => {
        expect(getBatteryVisual(80).colorClass).toContain("emerald");
        expect(getBatteryVisual(100).colorClass).toContain("emerald");
    });

    it("neutra entre 30 % y 79 %", () => {
        expect(getBatteryVisual(79).colorClass).toContain("gray");
        expect(getBatteryVisual(30).isLow).toBe(false);
    });

    it("baja por debajo de 30 %", () => {
        const visual = getBatteryVisual(29);
        expect(visual.isLow).toBe(true);
        expect(visual.colorClass).toContain("red");
    });

    it("sin dato cuando el nivel es null", () => {
        expect(getBatteryVisual(null).label).toBe("Sin dato");
    });

    it("redondea el porcentaje", () => {
        expect(getBatteryVisual(64.6).label).toBe("65%");
    });
});
