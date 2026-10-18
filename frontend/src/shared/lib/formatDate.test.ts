import { describe, expect, it } from "vitest";
import { formatAbsolute, formatRelative, formatTimestamp } from "./formatDate";

// Fechas construidas en hora local para que el resultado no dependa de la zona horaria de quien ejecuta.
const evening = new Date(2026, 9, 11, 21, 7, 15);
const now = evening.getTime();
const MINUTE = 60_000;

describe("formatAbsolute", () => {
    it("usa formato de 24 h", () => {
        expect(formatAbsolute(evening)).toBe("11/10/2026, 21:07:15");
    });

    it("muestra un guion si la fecha es inválida", () => {
        expect(formatAbsolute("no-es-fecha")).toBe("—");
    });
});

describe("formatRelative", () => {
    it.each([
        [30 * 1000, "hace instantes"],
        [5 * MINUTE, "hace 5 min"],
        [3 * 60 * MINUTE, "hace 3 h"],
    ])("hace %d ms → %s", (elapsed, expected) => {
        expect(formatRelative(now - elapsed, now)).toBe(expected);
    });
});

describe("formatTimestamp", () => {
    it("relativo si es de las últimas 24 h", () => {
        expect(formatTimestamp(now - 5 * MINUTE, now)).toBe("hace 5 min");
    });

    it("fecha corta en 24 h si es más antigua", () => {
        expect(formatTimestamp(evening, now + 2 * 24 * 60 * MINUTE)).toBe("11/10, 21:07");
    });
});
