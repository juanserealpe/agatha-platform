import { describe, expect, it } from "vitest";
import { EMPTY_FILTERS, historySearchKey, isIsoDay, parseHistorySearch, withHistorySearch } from "./historySearchParams";

const parse = (query: string) => parseHistorySearch(new URLSearchParams(query));

describe("parseHistorySearch", () => {
    it("sin parámetros: historial completo en la página 1", () => {
        expect(parse("")).toEqual({ filters: EMPTY_FILTERS, page: 1 });
    });

    it("lee todos los filtros", () => {
        expect(parse("dispositivo=11556&desde=2026-08-01&hasta=2026-08-31&nivel=ROJO,NARANJA&evento=FUGA&estado=SIN_CLASIFICAR&pagina=3")).toEqual({
            filters: {
                device: 11556,
                from: "2026-08-01",
                to: "2026-08-31",
                maxLevels: ["NARANJA", "ROJO"],
                events: ["FUGA"],
                statuses: ["SIN_CLASIFICAR"],
            },
            page: 3,
        });
    });

    it("ignora valores inválidos o manipulados", () => {
        expect(parse("dispositivo=abc&desde=2026-02-30&hasta=31/08/2026&nivel=ROJO,VERDE,rojo&evento=x&pagina=-2")).toEqual({
            filters: { ...EMPTY_FILTERS, maxLevels: ["ROJO"] },
            page: 1,
        });
    });
});

describe("withHistorySearch", () => {
    it("conserva parámetros ajenos al historial y omite la página 1", () => {
        const next = withHistorySearch(new URLSearchParams("alerta=123&pagina=4"), {
            filters: { ...EMPTY_FILTERS, events: ["MOVIMIENTO"] },
            page: 1,
        });
        expect(next.toString()).toBe("alerta=123&evento=MOVIMIENTO");
    });

    it("ida y vuelta sin pérdida", () => {
        const search = {
            filters: { ...EMPTY_FILTERS, device: 7, from: "2026-10-01", maxLevels: ["AMARILLO" as const, "ROJO" as const] },
            page: 2,
        };
        expect(parseHistorySearch(withHistorySearch(new URLSearchParams(), search))).toEqual(search);
    });
});

describe("historySearchKey", () => {
    it("no depende del orden en que se eligieron los valores", () => {
        expect(historySearchKey(parse("nivel=ROJO,AMARILLO"))).toBe(historySearchKey(parse("nivel=AMARILLO,ROJO")));
    });
});

describe("isIsoDay", () => {
    it.each([
        ["2026-08-24", true],
        ["2028-02-29", true],
        ["2026-02-29", false],
        ["2026-13-01", false],
        ["24/08/2026", false],
        ["", false],
    ])("%s → %s", (value, expected) => {
        expect(isIsoDay(value)).toBe(expected);
    });
});
