import { describe, expect, it } from "vitest";
import { exceedsExportLimit, exportFilename } from "./exportFile";

describe("exportFilename", () => {
    it("usa el nombre del servidor si llega", () => {
        expect(exportFilename("alertas_20261026_1504.xlsx")).toBe("alertas_20261026_1504.xlsx");
    });

    it("sin cabecera arma el nombre en hora de Colombia", () => {
        expect(exportFilename(null, new Date("2026-10-26T20:04:00Z"))).toBe("alertas_20261026_1504.xlsx");
    });
});

describe("exceedsExportLimit", () => {
    it("permite exactamente 10.000 y rechaza más", () => {
        expect(exceedsExportLimit(10_000)).toBe(false);
        expect(exceedsExportLimit(10_001)).toBe(true);
    });
});
