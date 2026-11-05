import { describe, expect, it } from "vitest";
import { paginationRange } from "./paginationRange";

describe("paginationRange", () => {
    it("muestra todas las páginas si son pocas", () => {
        expect(paginationRange(2, 3)).toEqual([1, 2, 3]);
        expect(paginationRange(1, 7)).toEqual([1, 2, 3, 4, 5, 6, 7]);
    });

    it("siempre al menos una página", () => {
        expect(paginationRange(1, 0)).toEqual([1]);
    });

    it("cerca del inicio", () => {
        expect(paginationRange(1, 12)).toEqual([1, 2, 3, 4, 5, "ellipsis-end", 12]);
        expect(paginationRange(4, 12)).toEqual([1, 2, 3, 4, 5, "ellipsis-end", 12]);
    });

    it("en el medio", () => {
        expect(paginationRange(6, 12)).toEqual([1, "ellipsis-start", 5, 6, 7, "ellipsis-end", 12]);
    });

    it("cerca del final", () => {
        expect(paginationRange(9, 12)).toEqual([1, "ellipsis-start", 8, 9, 10, 11, 12]);
        expect(paginationRange(12, 12)).toEqual([1, "ellipsis-start", 8, 9, 10, 11, 12]);
    });

    it("acota una página fuera de rango", () => {
        expect(paginationRange(99, 12)).toEqual([1, "ellipsis-start", 8, 9, 10, 11, 12]);
    });
});
