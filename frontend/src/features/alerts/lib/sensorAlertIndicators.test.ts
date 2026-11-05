import { describe, expect, it } from "vitest";
import type { ActiveDevice } from "@/services/alertManagement";
import { sensorAlertIndicators } from "./sensorAlertIndicators";

const point = (id: number, overrides: Partial<ActiveDevice> = {}): ActiveDevice => ({
    measuring_point_id: id,
    pk: `Sensor MP-${id}`,
    level: "AMARILLO",
    blinking: false,
    alerts: [],
    ...overrides,
});

describe("sensorAlertIndicators", () => {
    it("sin datos o sin alertas activas no marca ningún sensor", () => {
        expect(sensorAlertIndicators(undefined).size).toBe(0);
        expect(sensorAlertIndicators({ devices: [] }).size).toBe(0);
    });

    it("toma el nivel mayor y el titileo que resume el backend para cada punto", () => {
        const indicators = sensorAlertIndicators({
            devices: [point(11556, { level: "ROJO", blinking: true }), point(20, { level: "NARANJA" })],
        });
        expect(indicators.get(11556)).toMatchObject({ label: "Alerta roja", dotClass: "bg-alerta-roja", isBlinking: true });
        expect(indicators.get(20)).toMatchObject({ label: "Alerta naranja", dotClass: "bg-alerta-naranja", isBlinking: false });
        expect(indicators.has(30)).toBe(false);
    });
});
