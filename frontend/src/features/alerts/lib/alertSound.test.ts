import { describe, expect, it } from "vitest";
import { parseMuted, tonePattern } from "./alertSound";

describe("tonePattern", () => {
    it("es más insistente a mayor nivel: más notas y más agudas", () => {
        const yellow = tonePattern("AMARILLO");
        const orange = tonePattern("NARANJA");
        const red = tonePattern("ROJO");

        expect(yellow.length).toBeLessThan(orange.length);
        expect(orange.length).toBeLessThan(red.length);
        expect(yellow[0].frequency).toBeLessThan(orange[0].frequency);
        expect(orange[0].frequency).toBeLessThan(red[0].frequency);
    });

    it("las notas no se pisan", () => {
        for (const level of ["AMARILLO", "NARANJA", "ROJO"] as const) {
            const pattern = tonePattern(level);
            pattern.slice(1).forEach((beep, i) => {
                expect(beep.start).toBeGreaterThanOrEqual(pattern[i].start + pattern[i].duration);
            });
        }
    });
});

describe("parseMuted", () => {
    it("solo 'true' silencia; sin valor guardado suena", () => {
        expect(parseMuted("true")).toBe(true);
        expect(parseMuted("false")).toBe(false);
        expect(parseMuted(null)).toBe(false);
        expect(parseMuted("x")).toBe(false);
    });
});
