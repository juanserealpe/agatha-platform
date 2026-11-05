import { describe, expect, it } from "vitest";
import { isDemoPath } from "./appMode";

describe("isDemoPath", () => {
    it("reconoce /demo y las rutas debajo", () => {
        expect(isDemoPath("/demo")).toBe(true);
        expect(isDemoPath("/demo/")).toBe(true);
        expect(isDemoPath("/demo/monitoreo")).toBe(true);
        expect(isDemoPath("/demo/historial")).toBe(true);
    });

    it("no confunde otras rutas con la demo", () => {
        expect(isDemoPath("/")).toBe(false);
        expect(isDemoPath("/monitoreo")).toBe(false);
        expect(isDemoPath("/demostracion")).toBe(false);
        expect(isDemoPath("/historial/demo")).toBe(false);
    });
});
