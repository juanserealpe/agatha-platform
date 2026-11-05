import { describe, expect, it } from "vitest";
import {
    formatAbsolute,
    formatClock,
    formatColombiaDate,
    formatColombiaFileStamp,
    formatColombiaDateTime,
    formatDayMonth,
    formatRelative,
    formatShort,
    formatTimestamp,
} from "./formatDate";

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

describe("formatos de gráficas", () => {
    it("formatShort es siempre absoluto aunque la fecha sea reciente", () => {
        expect(formatShort(evening)).toBe("11/10, 21:07");
    });

    it("formatClock muestra solo la hora en 24 h", () => {
        expect(formatClock(new Date(2026, 9, 11, 0, 5))).toBe("00:05");
    });

    it("formatDayMonth omite el conector y el punto del mes", () => {
        expect(formatDayMonth(evening)).toBe("11 oct");
    });
});

describe("hora de Colombia", () => {
    it("formatColombiaDateTime respeta el offset del backend", () => {
        expect(formatColombiaDateTime("2026-08-24T11:00:00-05:00")).toBe("24/08/2026 11:00");
    });

    it("convierte UTC a Colombia aunque cambie el día", () => {
        expect(formatColombiaDateTime("2026-08-24T03:30:00Z")).toBe("23/08/2026 22:30");
        expect(formatColombiaDate("2026-08-24T03:30:00Z")).toBe("23/08/2026");
    });

    it("muestra un guion si la fecha es inválida", () => {
        expect(formatColombiaDateTime("no-es-fecha")).toBe("—");
        expect(formatColombiaDate("no-es-fecha")).toBe("—");
    });
});

describe("formatColombiaFileStamp", () => {
    it("usa la hora de Colombia", () => {
        expect(formatColombiaFileStamp("2026-10-26T20:04:00Z")).toBe("20261026_1504");
        expect(formatColombiaFileStamp("2026-10-27T02:30:00Z")).toBe("20261026_2130");
    });

    it("vacío si la fecha es inválida", () => {
        expect(formatColombiaFileStamp("no-es-fecha")).toBe("");
    });
});
