import { describe, expect, it } from "vitest";
import { DEFAULT_LEAK_THRESHOLDS } from "@/features/telemetry/config/leakThresholds";
import { isApiError } from "@/services/http/apiError";
import type { LevelInterval } from "./alertStore";
import { DEMO_THRESHOLDS } from "./demoData";
import { generateSeries, type DemoSeriesRequest } from "./series";

const NOW = new Date("2026-10-31T15:00:00Z");

const ALERT_INTERVALS: LevelInterval[] = [
    { level: "AMARILLO", from: new Date("2026-10-31T06:00:00Z"), to: new Date("2026-10-31T10:00:00Z") },
    { level: "NARANJA", from: new Date("2026-10-31T10:00:00Z"), to: new Date("2026-10-31T13:00:00Z") },
    { level: "ROJO", from: new Date("2026-10-31T13:00:00Z"), to: null },
];

function request(overrides: Partial<DemoSeriesRequest> = {}): DemoSeriesRequest {
    return {
        deviceId: 9001,
        startDate: "2026-10-30T15:00:00.000Z",
        endDate: "2026-10-31T15:00:00.000Z",
        variables: ["indicador", "b2"],
        intervals: ALERT_INTERVALS,
        now: NOW,
        ...overrides,
    };
}

function valueAt(response: ReturnType<typeof generateSeries>, key: string, timestamp: string): number | undefined {
    return response.series.find((series) => series.key === key)?.data.find((point) => point.timestamp === timestamp)?.value;
}

/** Nivel en que cae un valor del indicador según las franjas de la gráfica. */
function band(value: number): string {
    if (value >= DEFAULT_LEAK_THRESHOLDS.rojo) return "rojo";
    if (value >= DEFAULT_LEAK_THRESHOLDS.naranja) return "naranja";
    if (value >= DEFAULT_LEAK_THRESHOLDS.amarillo) return "amarillo";
    return "normal";
}

describe("generateSeries", () => {
    it("usa las mismas franjas que la gráfica del indicador", () => {
        expect(DEMO_THRESHOLDS).toEqual(DEFAULT_LEAK_THRESHOLDS);
    });

    it("devuelve las claves en el orden del catálogo, con nombre, unidad y origen del backend", () => {
        const response = generateSeries(request());
        expect(response.series.map(({ key, name, unit, source }) => ({ key, name, unit, source }))).toEqual([
            { key: "b2", name: "B2 fuga 1200-2000Hz", unit: "-", source: "measurement" },
            { key: "indicador", name: "Indicador I (24 h)", unit: "-", source: "evaluation" },
        ]);
        expect(generateSeries(request({ variables: [] })).series).toHaveLength(12);
    });

    it("rechaza una variable desconocida como el backend", () => {
        try {
            generateSeries(request({ variables: ["b3"] }));
            expect.unreachable();
        } catch (error) {
            expect(isApiError(error) && error.status).toBe(400);
        }
    });

    it("muestrea cada 15 min la telemetría y cada hora las evaluaciones, sin pasar de ahora", () => {
        const response = generateSeries(request({ endDate: "2026-11-01T00:00:00.000Z" }));
        const [b2, indicator] = response.series;
        expect(b2.data).toHaveLength(24 * 4 + 1);
        expect(indicator.data).toHaveLength(24 + 1);
        expect(indicator.data.at(-1)?.timestamp).toBe(NOW.toISOString());
    });

    it("el indicador cae en la franja del nivel de la alerta y fuera de ella es normal", () => {
        const response = generateSeries(request());
        expect(band(valueAt(response, "indicador", "2026-10-31T03:00:00.000Z")!)).toBe("normal");
        expect(band(valueAt(response, "indicador", "2026-10-31T08:00:00.000Z")!)).toBe("amarillo");
        expect(band(valueAt(response, "indicador", "2026-10-31T12:00:00.000Z")!)).toBe("naranja");
        expect(band(valueAt(response, "indicador", "2026-10-31T15:00:00.000Z")!)).toBe("rojo");
    });

    it("B2 sube de forma visible durante la alerta", () => {
        const normal = generateSeries(request({ intervals: [] }));
        const alert = generateSeries(request());
        const at = "2026-10-31T14:30:00.000Z";
        expect(valueAt(alert, "b2", at)!).toBeGreaterThan(2 * valueAt(normal, "b2", at)!);
    });

    it("es determinista: el mismo instante da el mismo valor", () => {
        expect(generateSeries(request())).toEqual(generateSeries(request()));
    });
});
