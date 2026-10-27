import { describe, expect, it } from "vitest";
import { EMPTY_FILTERS } from "./historySearchParams";
import {
    dateToIsoDay,
    draftFromFilters,
    isoDayToDate,
    isoDayToDisplay,
    parseDisplayDay,
    RANGE_ERROR_MESSAGE,
    validateDraft,
    type FilterDraft,
} from "./historyFilterForm";
import { summarizeFilters } from "./filterSummary";

const EMPTY_DRAFT: FilterDraft = draftFromFilters(EMPTY_FILTERS);

describe("parseDisplayDay", () => {
    it.each([
        ["24/08/2026", "2026-08-24"],
        ["4/8/2026", "2026-08-04"],
        [" 01/10/2026 ", "2026-10-01"],
        ["2026-08-24", "2026-08-24"],
    ])("%s → %s", (text, expected) => {
        expect(parseDisplayDay(text)).toBe(expected);
    });

    it.each(["31/02/2026", "24-08-2026", "24/08/26", "hoy", ""])("%s → null", (text) => {
        expect(parseDisplayDay(text)).toBeNull();
    });
});

describe("conversiones de día", () => {
    it("ida y vuelta entre texto, ISO y fecha local", () => {
        expect(isoDayToDisplay("2026-08-04")).toBe("04/08/2026");
        expect(dateToIsoDay(isoDayToDate("2026-08-04"))).toBe("2026-08-04");
    });
});

describe("validateDraft", () => {
    it("sin filtros: historial completo", () => {
        expect(validateDraft(EMPTY_DRAFT)).toEqual({ ok: true, filters: EMPTY_FILTERS });
    });

    it("convierte las fechas a aaaa-mm-dd", () => {
        const result = validateDraft({ ...EMPTY_DRAFT, fromText: "01/08/2026", toText: "31/08/2026", events: ["FUGA"] });
        expect(result).toEqual({ ok: true, filters: { ...EMPTY_FILTERS, from: "2026-08-01", to: "2026-08-31", events: ["FUGA"] } });
    });

    it("con una sola fecha el rango queda abierto", () => {
        expect(validateDraft({ ...EMPTY_DRAFT, toText: "31/08/2026" })).toMatchObject({ ok: true, filters: { from: null, to: "2026-08-31" } });
    });

    it("el mismo día en ambas fechas es válido", () => {
        expect(validateDraft({ ...EMPTY_DRAFT, fromText: "24/08/2026", toText: "24/08/2026" }).ok).toBe(true);
    });

    it("rechaza fecha fin anterior a fecha inicio", () => {
        expect(validateDraft({ ...EMPTY_DRAFT, fromText: "31/08/2026", toText: "01/08/2026" })).toEqual({
            ok: false,
            field: "to",
            message: RANGE_ERROR_MESSAGE,
        });
    });

    it("señala la fecha mal escrita", () => {
        expect(validateDraft({ ...EMPTY_DRAFT, fromText: "32/08/2026" })).toMatchObject({ ok: false, field: "from" });
        expect(validateDraft({ ...EMPTY_DRAFT, toText: "agosto" })).toMatchObject({ ok: false, field: "to" });
    });
});

describe("summarizeFilters", () => {
    const deviceName = (id: number) => (id === 11556 ? "PK34+500" : `Dispositivo ${id}`);

    it("vacío con el historial completo", () => {
        expect(summarizeFilters(EMPTY_FILTERS, deviceName)).toEqual([]);
    });

    it("resume cada filtro aplicado", () => {
        expect(
            summarizeFilters(
                {
                    device: 11556,
                    from: "2026-10-01",
                    to: "2026-10-08",
                    maxLevels: ["NARANJA", "ROJO"],
                    events: ["FUGA"],
                    statuses: ["SIN_CLASIFICAR"],
                },
                deviceName
            )
        ).toEqual(["PK34+500", "01/10/2026 – 08/10/2026", "Naranja, Rojo", "Fuga", "Sin clasificar"]);
    });

    it("rango abierto", () => {
        expect(summarizeFilters({ ...EMPTY_FILTERS, from: "2026-10-01" }, deviceName)).toEqual(["Desde 01/10/2026"]);
        expect(summarizeFilters({ ...EMPTY_FILTERS, to: "2026-10-08" }, deviceName)).toEqual(["Hasta 08/10/2026"]);
    });
});
