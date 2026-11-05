import { describe, expect, it } from "vitest";
import { getDeviceStatus } from "./sensorStatus";

const NOW = new Date("2026-10-18T12:00:00Z").getTime();
const minutesAgo = (minutes: number) => new Date(NOW - minutes * 60_000).toISOString();

describe("getDeviceStatus", () => {
    it("en línea hasta 20 min sin reportar", () => {
        expect(getDeviceStatus(minutesAgo(20), NOW)).toBe("online");
    });

    it("retrasado entre 20 min y 2 h", () => {
        expect(getDeviceStatus(minutesAgo(21), NOW)).toBe("delayed");
        expect(getDeviceStatus(minutesAgo(120), NOW)).toBe("delayed");
    });

    it("sin comunicación pasadas 2 h o si nunca reportó", () => {
        expect(getDeviceStatus(minutesAgo(121), NOW)).toBe("offline");
        expect(getDeviceStatus(null, NOW)).toBe("offline");
    });
});
