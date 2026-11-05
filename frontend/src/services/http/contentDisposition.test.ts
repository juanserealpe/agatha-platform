import { describe, expect, it } from "vitest";
import { parseContentDispositionFilename } from "./contentDisposition";

describe("parseContentDispositionFilename", () => {
    it.each([
        ['attachment; filename="alertas_20261026_1504.xlsx"', "alertas_20261026_1504.xlsx"],
        ["attachment; filename=alertas.xlsx", "alertas.xlsx"],
        ["attachment; filename*=UTF-8''alertas%20oct.xlsx", "alertas oct.xlsx"],
        ['attachment; filename="viejo.xlsx"; filename*=UTF-8\'\'nuevo.xlsx', "nuevo.xlsx"],
    ])("%s → %s", (header, expected) => {
        expect(parseContentDispositionFilename(header)).toBe(expected);
    });

    it("devuelve null sin cabecera o sin nombre", () => {
        expect(parseContentDispositionFilename(null)).toBeNull();
        expect(parseContentDispositionFilename("attachment")).toBeNull();
        expect(parseContentDispositionFilename('attachment; filename=""')).toBeNull();
    });

    it("no permite rutas en el nombre", () => {
        expect(parseContentDispositionFilename('attachment; filename="../../etc/alertas.xlsx"')).toBe(".._.._etc_alertas.xlsx");
    });

    it("usa filename simple si filename* está mal codificado", () => {
        expect(parseContentDispositionFilename("attachment; filename*=UTF-8''%E0%A4%A; filename=alertas.xlsx")).toBe("alertas.xlsx");
    });
});
