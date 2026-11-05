import { describe, expect, it } from "vitest";
import type { Series } from "@/services/dashboard/series.types";
import { buildChartRows, computeTimeTicks, computeYDomain, computeYTicks, isDayTick, nearestIndex, niceStep, summarize } from "./chartData";

const MIN = 60_000;
const HOUR = 60 * MIN;
const t0 = Date.UTC(2026, 9, 19, 12, 0, 0);

function series(key: string, source: Series["source"], points: [number, number][]): Series {
    return {
        key, name: key, unit: "-", source,
        data: points.map(([offset, value]) => ({ timestamp: new Date(t0 + offset).toISOString(), value })),
    };
}

describe("buildChartRows", () => {
    it("une en una fila las lecturas de varias series del mismo instante aunque difieran en segundos", () => {
        const b1 = series("b1", "measurement", [[0, 10]]);
        const b2 = series("b2", "measurement", [[3_000, 50]]);
        expect(buildChartRows([b1, b2])).toEqual([{ timestamp: t0, b1: 10, b2: 50 }]);
    });

    it("corta la línea con una fila vacía cuando falta más de 2,5 lecturas seguidas", () => {
        const s = series("b2", "measurement", [[0, 1], [15 * MIN, 2], [90 * MIN, 3]]);
        const rows = buildChartRows([s]);
        expect(rows).toHaveLength(4);
        expect(rows[2]).toEqual({ timestamp: t0 + 52.5 * MIN, b2: null });
    });

    it("no corta un retraso normal de la lectura", () => {
        const s = series("b2", "measurement", [[0, 1], [30 * MIN, 2]]);
        expect(buildChartRows([s])).toHaveLength(2);
    });

    it("mide los huecos con la cadencia horaria en series calculadas", () => {
        const s = series("indicador", "evaluation", [[0, 0.1], [2 * HOUR, 0.1], [5 * HOUR, 0.1]]);
        expect(buildChartRows([s]).filter((r) => r.indicador === null)).toHaveLength(1);
    });
});

describe("summarize", () => {
    it("último, mínimo y máximo", () => {
        const s = series("x", "measurement", [[0, 5], [MIN, -2], [2 * MIN, 9], [3 * MIN, 4]]);
        const summary = summarize(s.data);
        expect(summary?.last.value).toBe(4);
        expect([summary?.min, summary?.max]).toEqual([-2, 9]);
    });

    it("null sin datos", () => {
        expect(summarize([])).toBeNull();
    });
});

describe("computeYDomain", () => {
    it("mantiene visibles los umbrales aunque los datos estén lejos (indicador)", () => {
        const [lo, hi] = computeYDomain({
            min: 0.02, max: 0.1,
            scale: { min: 0, minTop: 0.4, max: 1 },
            thresholds: [{ label: "Rojo", from: 0.35, color: "#000" }],
        });
        expect(lo).toBe(0);
        expect(hi).toBeGreaterThanOrEqual(0.4);
        expect(hi).toBeLessThanOrEqual(1);
    });

    it("redondea los bordes a un paso legible", () => {
        expect(computeYDomain({ min: 35, max: 470 })).toEqual([0, 600]);
        expect(computeYDomain({ min: 0, max: 13.68, scale: { min: 0, minTop: 25, max: 100 } })).toEqual([0, 30]);
        expect(computeYDomain({ min: 0.02, max: 0.1, scale: { min: 0, minTop: 0.4, max: 1 } })).toEqual([0, 0.4]);
    });

    it("niceStep elige 1 · 2 · 2,5 · 3 · 5 × 10ⁿ", () => {
        expect([niceStep(0.07), niceStep(1.3), niceStep(2.2), niceStep(260), niceStep(130)]).toEqual([0.1, 2, 2.5, 300, 200]);
    });

    it("con rango mínimo, un cambio de 0,06° no ocupa toda la altura", () => {
        const [lo, hi] = computeYDomain({ min: 25.45, max: 25.51, scale: { minSpan: 2 } });
        expect(hi - lo).toBeGreaterThanOrEqual(2);
        expect(lo).toBeLessThan(25.45);
        expect(hi).toBeGreaterThan(25.51);
    });

    it("el rango mínimo no cruza a negativo una variable que no lo es", () => {
        expect(computeYDomain({ min: 0, max: 0.01, scale: { min: 0, minSpan: 1 } })[0]).toBe(0);
    });

    it("respeta el techo fijo aunque haya picos", () => {
        expect(computeYDomain({ min: 0, max: 100, scale: { max: 100 } })[1]).toBe(100);
    });

    it("da margen a una serie constante en vez de colapsar el eje", () => {
        const [lo, hi] = computeYDomain({ min: -3.17, max: -3.17 });
        expect(lo).toBeLessThan(-3.17);
        expect(hi).toBeGreaterThan(-3.17);
    });

    it("sin datos usa la escala configurada", () => {
        expect(computeYDomain({ min: Infinity, max: -Infinity, scale: { min: 0, minTop: 25 } })).toEqual([0, 25]);
    });
});

describe("computeTimeTicks", () => {
    it("marcas en horas exactas y como mucho 7", () => {
        const start = new Date(2026, 9, 19, 5, 47).getTime();
        const { ticks, stepMs } = computeTimeTicks(start, start + 24 * HOUR);
        expect(ticks.length).toBeLessThanOrEqual(7);
        expect(stepMs).toBe(6 * HOUR);
        for (const t of ticks) expect(new Date(t).getMinutes()).toBe(0);
        expect(ticks[0]).toBeGreaterThanOrEqual(start);
    });

    it("una semana se marca por días", () => {
        const start = new Date(2026, 9, 12, 13, 0).getTime();
        const { ticks, stepMs } = computeTimeTicks(start, start + 7 * 24 * HOUR);
        expect(isDayTick(ticks[0], stepMs)).toBe(true);
        expect(new Date(ticks[0]).getHours()).toBe(0);
    });

    it("en pasos de horas solo la medianoche muestra fecha", () => {
        expect(isDayTick(new Date(2026, 9, 19, 0, 0).getTime(), HOUR)).toBe(true);
        expect(isDayTick(new Date(2026, 9, 19, 6, 0).getTime(), HOUR)).toBe(false);
    });
});

describe("nearestIndex", () => {
    const hourly = [0, HOUR, 2 * HOUR, 3 * HOUR];

    it("busca el punto horario más cercano a una lectura de 15 min", () => {
        expect(nearestIndex(hourly, HOUR + 20 * MIN)).toBe(1);
        expect(nearestIndex(hourly, HOUR + 40 * MIN)).toBe(2);
    });

    it("-1 si no hay punto comparable cerca", () => {
        expect(nearestIndex(hourly, 6 * HOUR)).toBe(-1);
        expect(nearestIndex([], 0)).toBe(-1);
    });
});

describe("computeYTicks", () => {
    it("marcas redondas dentro del dominio", () => {
        expect(computeYTicks([0, 30])).toEqual([0, 10, 20, 30]);
        expect(computeYTicks([0, 0.4])).toEqual([0, 0.1, 0.2, 0.3, 0.4]);
        expect(computeYTicks([-3.5, -2.5])).toEqual([-3.5, -3.25, -3, -2.75, -2.5]);
    });
});
