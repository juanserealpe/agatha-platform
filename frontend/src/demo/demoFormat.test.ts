import { describe, expect, it } from "vitest";
import { colombiaDay, floorToHour, formatHour, formatValue, toColombiaIso, toUtcIso } from "./demoFormat";

const instant = new Date("2026-08-25T03:30:15.250Z");

describe("demoFormat", () => {
    it("formatea en UTC y en hora de Colombia, sin milisegundos", () => {
        expect(toUtcIso(instant)).toBe("2026-08-25T03:30:15Z");
        expect(toColombiaIso(instant)).toBe("2026-08-24T22:30:15-05:00");
    });

    it("usa el día de Colombia, no el de UTC", () => {
        expect(colombiaDay(instant)).toBe("2026-08-24");
    });

    it("escribe la hora como el catálogo del backend", () => {
        expect(formatHour(instant)).toBe("22:30 del 24/08");
    });

    it("escribe los índices con coma y un tercer decimal solo si hace falta", () => {
        expect(formatValue(0.16)).toBe("0,16");
        expect(formatValue(0.158)).toBe("0,158");
        expect(formatValue(0.3)).toBe("0,30");
    });

    it("redondea al inicio de la hora", () => {
        expect(floorToHour(instant).toISOString()).toBe("2026-08-25T03:00:00.000Z");
    });
});
