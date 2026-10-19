import { describe, expect, it } from "vitest";
import { shouldPulse } from "./alertDisplay";

describe("shouldPulse", () => {
    it("pulsa solo la alerta roja sin clasificar", () => {
        expect(shouldPulse({ severity: "red", status: "unclassified" })).toBe(true);
        expect(shouldPulse({ severity: "red", status: "confirmed" })).toBe(false);
        expect(shouldPulse({ severity: "orange", status: "unclassified" })).toBe(false);
    });
});
