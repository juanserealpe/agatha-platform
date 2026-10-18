import { describe, expect, it } from "vitest";
import { emptyAlertsService } from "./emptyAlerts.adapter";

describe("emptyAlertsService", () => {
    it("no devuelve alertas mientras el backend no las exponga", async () => {
        await expect(emptyAlertsService.getRecent(5)).resolves.toEqual([]);
        await expect(emptyAlertsService.getPending()).resolves.toEqual([]);
        await expect(emptyAlertsService.getById("cualquiera")).resolves.toBeNull();
    });
});
