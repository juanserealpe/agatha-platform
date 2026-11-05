import { describe, expect, it } from "vitest";
import { extractPk, formatPk, parsePkQuery, pkMatchesQuery } from "./pk";

describe("extractPk", () => {
    it("lee el PK del nombre del dispositivo con guion o con +", () => {
        expect(extractPk("Sensor MP-11556 PK34-470")).toBe(34470);
        expect(extractPk("Punto 1 · PK 34+470")).toBe(34470);
    });

    it("devuelve null si el texto no contiene un PK", () => {
        expect(extractPk("Sensor MP-11556")).toBeNull();
    });
});

describe("formatPk", () => {
    it("formatea metros como PK con km de 2 dígitos y metros de 3", () => {
        expect(formatPk(34470)).toBe("PK 34+470");
        expect(formatPk(2100)).toBe("PK 02+100");
        expect(formatPk(0)).toBe("PK 00+000");
    });
});

describe("parsePkQuery", () => {
    it.each([
        ["34+470", 34, "470"],
        ["34-470", 34, "470"],
        ["PK 34+470", 34, "470"],
        ["34 470", 34, "470"],
        ["34+", 34, ""],
        ["34+4", 34, "4"],
    ])("con separador: %s", (input, km, metersPrefix) => {
        expect(parsePkQuery(input)).toEqual({ kind: "parts", km, metersPrefix });
    });

    it.each(["3", "344", "34470", "pk344"])("solo dígitos: %s", (input) => {
        expect(parsePkQuery(input)).toEqual({ kind: "digits", digits: input.replace(/^pk/, "") });
    });

    it.each(["", "sensor", "1234567"])("descarta %j", (input) => {
        expect(parsePkQuery(input)).toBeNull();
    });
});

describe("pkMatchesQuery", () => {
    it("con separador: km exacto y prefijo de metros", () => {
        expect(pkMatchesQuery(34470, { kind: "parts", km: 34, metersPrefix: "" })).toBe(true);
        expect(pkMatchesQuery(34470, { kind: "parts", km: 34, metersPrefix: "47" })).toBe(true);
        expect(pkMatchesQuery(34470, { kind: "parts", km: 35, metersPrefix: "" })).toBe(false);
        expect(pkMatchesQuery(34470, { kind: "parts", km: 34, metersPrefix: "471" })).toBe(false);
    });

    it.each(["3", "34", "344", "3447", "34470"])("solo dígitos: %s es prefijo de 34+470", (digits) => {
        expect(pkMatchesQuery(34470, { kind: "digits", digits })).toBe(true);
    });

    it.each(["345", "34471", "4"])("solo dígitos: %s no es prefijo de 34+470", (digits) => {
        expect(pkMatchesQuery(34470, { kind: "digits", digits })).toBe(false);
    });

    it("acepta el km con o sin cero a la izquierda", () => {
        expect(pkMatchesQuery(2100, { kind: "digits", digits: "21" })).toBe(true);
        expect(pkMatchesQuery(2100, { kind: "digits", digits: "021" })).toBe(true);
    });
});
