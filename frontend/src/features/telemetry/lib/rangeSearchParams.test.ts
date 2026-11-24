import { describe, expect, it } from "vitest";
import { customSelectionFrom, customWindow, parseRangeSelection, withRangeSelection } from "./rangeSearchParams";

const NOW = new Date(2026, 10, 24, 15, 30).getTime();
const params = (query: string) => new URLSearchParams(query);

describe("parseRangeSelection", () => {
    it("sin parámetros usa el rango por defecto (1 semana)", () => {
        expect(parseRangeSelection(params(""), NOW)).toEqual({ kind: "quick", range: "1w" });
    });

    it("lee un rango rápido por su valor de URL", () => {
        expect(parseRangeSelection(params("rango=6h"), NOW)).toEqual({ kind: "quick", range: "6h" });
        expect(parseRangeSelection(params("rango=1mes"), NOW)).toEqual({ kind: "quick", range: "1m" });
    });

    it("un rango rápido desconocido cae al de por defecto", () => {
        expect(parseRangeSelection(params("rango=2años"), NOW)).toEqual({ kind: "quick", range: "1w" });
    });

    it("lee días personalizados, que mandan sobre ?rango=", () => {
        expect(parseRangeSelection(params("rango=1h&desde=2026-11-01&hasta=2026-11-10"), NOW)).toEqual({
            kind: "custom",
            start: new Date(2026, 10, 1),
            end: new Date(2026, 10, 10),
        });
    });

    it("descarta días imposibles, al revés, incompletos o en el futuro", () => {
        for (const query of [
            "desde=2026-02-31&hasta=2026-03-02",
            "desde=2026-11-10&hasta=2026-11-01",
            "desde=2026-11-01",
            "desde=2026-12-01&hasta=2026-12-05",
            "desde=1-11-2026&hasta=2026-11-05",
        ]) {
            expect(parseRangeSelection(params(query), NOW)).toEqual({ kind: "quick", range: "1w" });
        }
    });
});

describe("withRangeSelection", () => {
    it("conserva los demás parámetros y no escribe el rango por defecto", () => {
        const next = withRangeSelection(params("alerta=12&rango=6h"), { kind: "quick", range: "1w" });
        expect(next.toString()).toBe("alerta=12");
    });

    it("escribe un rango rápido y borra los días personalizados", () => {
        const next = withRangeSelection(params("desde=2026-11-01&hasta=2026-11-10"), { kind: "quick", range: "1d" });
        expect(next.toString()).toBe("rango=1d");
    });

    it("escribe días personalizados en formato AAAA-MM-DD", () => {
        const next = withRangeSelection(params("rango=6h"), {
            kind: "custom",
            start: new Date(2026, 0, 5),
            end: new Date(2026, 0, 9),
        });
        expect(next.toString()).toBe("desde=2026-01-05&hasta=2026-01-09");
    });

    it("ida y vuelta: lo escrito se lee igual", () => {
        const selection = { kind: "custom", start: new Date(2026, 10, 3), end: new Date(2026, 10, 20) } as const;
        expect(parseRangeSelection(withRangeSelection(params(""), selection), NOW)).toEqual(selection);
    });
});

describe("rango personalizado", () => {
    it("customSelectionFrom pasa del rango aplicado a sus días locales", () => {
        const range = { startDate: new Date(2026, 10, 3).toISOString(), endDate: new Date(2026, 10, 20, 23, 59).toISOString() };
        expect(customSelectionFrom(range)).toEqual({ kind: "custom", start: new Date(2026, 10, 3), end: new Date(2026, 10, 20) });
    });

    it("customWindow llega al final del último día pero no pasa de ahora", () => {
        const past = customWindow({ kind: "custom", start: new Date(2026, 10, 1), end: new Date(2026, 10, 2) }, NOW);
        expect(past.endDate).toBe(new Date(2026, 10, 2, 23, 59, 59, 999).toISOString());
        const today = customWindow({ kind: "custom", start: new Date(2026, 10, 20), end: new Date(2026, 10, 24) }, NOW);
        expect(today.endDate).toBe(new Date(NOW).toISOString());
    });
});
