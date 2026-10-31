import { describe, expect, it } from "vitest";
import { primaryManagementMode } from "./managementMode";

describe("primaryManagementMode", () => {
    it("abre en la gestión pendiente que indica el backend", () => {
        expect(primaryManagementMode(["CONFIRM", "FALSE_ALERT", "VIEW"])).toBe("clasificar");
        expect(primaryManagementMode(["ADD_LABEL", "VIEW"])).toBe("etiquetar");
    });

    it("en lectura si no queda nada por gestionar", () => {
        expect(primaryManagementMode(["VIEW"])).toBe("lectura");
        expect(primaryManagementMode([])).toBe("lectura");
    });
});
