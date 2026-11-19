import { describe, expect, it } from "vitest";
import { draftFromApplied, isComplete, pickDay, toAppliedRange } from "./customRange";

const day = (d: number, h = 0) => new Date(2026, 10, d, h);

describe("pickDay", () => {
    it("al elegir el inicio pasa a pedir el fin", () => {
        expect(pickDay({}, "start", day(10, 15))).toEqual({ draft: { start: day(10), end: undefined }, next: "end" });
    });

    it("completa el rango al elegir el fin", () => {
        expect(pickDay({ start: day(10) }, "end", day(14))).toEqual({
            draft: { start: day(10), end: day(14) },
            next: "end",
        });
    });

    it("permite un rango de un solo día", () => {
        expect(pickDay({ start: day(10) }, "end", day(10)).draft).toEqual({ start: day(10), end: day(10) });
    });

    it("un fin anterior al inicio se toma como nuevo inicio", () => {
        expect(pickDay({ start: day(10), end: day(14) }, "end", day(5))).toEqual({
            draft: { start: day(5), end: undefined },
            next: "end",
        });
    });

    it("cambiar el inicio conserva el fin si sigue siendo válido", () => {
        expect(pickDay({ start: day(10), end: day(14) }, "start", day(12)).draft).toEqual({
            start: day(12),
            end: day(14),
        });
    });

    it("cambiar el inicio a después del fin descarta el fin", () => {
        expect(pickDay({ start: day(10), end: day(14) }, "start", day(20))).toEqual({
            draft: { start: day(20), end: undefined },
            next: "end",
        });
    });

    it("si se elige primero el fin, pasa a pedir el inicio", () => {
        expect(pickDay({}, "end", day(14))).toEqual({ draft: { start: undefined, end: day(14) }, next: "start" });
    });
});

describe("isComplete", () => {
    it("exige ambos extremos", () => {
        expect(isComplete({ start: day(1) })).toBe(false);
        expect(isComplete({ start: day(1), end: day(2) })).toBe(true);
    });
});

describe("toAppliedRange", () => {
    it("cubre desde el inicio del primer día hasta el final del último", () => {
        const now = day(30).getTime();
        expect(toAppliedRange({ start: day(10), end: day(14) }, now)).toEqual({
            startDate: day(10).toISOString(),
            endDate: new Date(2026, 10, 14, 23, 59, 59, 999).toISOString(),
        });
    });

    it("no pasa de ahora, redondeado al minuto", () => {
        const now = new Date(2026, 10, 14, 9, 30, 45).getTime();
        expect(toAppliedRange({ start: day(10), end: day(14) }, now).endDate).toBe(
            new Date(2026, 10, 14, 9, 30).toISOString()
        );
    });
});

describe("draftFromApplied", () => {
    it("vuelve a los días del calendario", () => {
        const applied = toAppliedRange({ start: day(10), end: day(14) }, day(30).getTime());
        expect(draftFromApplied(applied)).toEqual({ start: day(10), end: day(14) });
    });

    it("sin rango aplicado empieza vacío", () => {
        expect(draftFromApplied(null)).toEqual({});
    });
});
