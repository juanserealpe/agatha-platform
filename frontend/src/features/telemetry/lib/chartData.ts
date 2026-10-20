import type { Series, SeriesPoint } from "@/services/dashboard/series.types";
import type { ReferenceMark, ThresholdBand, YScale } from "../config/seriesSections";

const MINUTE_MS = 60_000;
const HOUR_MS = 60 * MINUTE_MS;
const DAY_MS = 24 * HOUR_MS;

// Lecturas del sensor cada ~15 min; indicador y densidad una por hora.
const CADENCE_MS: Record<Series["source"], number> = { measurement: 15 * MINUTE_MS, evaluation: HOUR_MS };
// Una separación mayor a esto entre dos puntos es un hueco real: la línea se corta en vez de unirlos.
const GAP_FACTOR = 2.5;
// Las lecturas llegan con segundos distintos por serie; se agrupan para alinearlas en una misma fila.
const BUCKET_MS = 10_000;
const Y_PADDING_RATIO = 0.1;

export type ChartRow = { timestamp: number } & Record<string, number | null>;

/**
 * Une las series en filas por instante y agrega una fila vacía en cada hueco para que la línea se
 * corte: así un periodo sin datos no se dibuja como si el valor se hubiera mantenido.
 */
export function buildChartRows(series: Series[]): ChartRow[] {
    const byBucket = new Map<number, ChartRow>();
    for (const s of series) {
        for (const point of s.data) {
            const bucket = Math.round(new Date(point.timestamp).getTime() / BUCKET_MS) * BUCKET_MS;
            const row = byBucket.get(bucket) ?? { timestamp: bucket };
            row[s.key] = point.value;
            byBucket.set(bucket, row);
        }
    }
    const rows = [...byBucket.values()].sort((a, b) => a.timestamp - b.timestamp);
    if (rows.length < 2) return rows;

    const maxGapMs = Math.max(...series.map((s) => CADENCE_MS[s.source])) * GAP_FACTOR;
    const emptyRow = (timestamp: number): ChartRow => {
        const row: ChartRow = { timestamp };
        for (const s of series) row[s.key] = null;
        return row;
    };

    const withGaps: ChartRow[] = [rows[0]];
    for (let i = 1; i < rows.length; i++) {
        const prev = rows[i - 1];
        if (rows[i].timestamp - prev.timestamp > maxGapMs) {
            withGaps.push(emptyRow((prev.timestamp + rows[i].timestamp) / 2));
        }
        withGaps.push(rows[i]);
    }
    return withGaps;
}

export interface SeriesSummary {
    last: SeriesPoint;
    min: number;
    max: number;
}

/** Último valor, mínimo y máximo del rango. Bucle en vez de `Math.min(...xs)`: un mes de lecturas desborda la pila. */
export function summarize(points: SeriesPoint[]): SeriesSummary | null {
    if (points.length === 0) return null;
    let min = Infinity;
    let max = -Infinity;
    for (const p of points) {
        if (p.value < min) min = p.value;
        if (p.value > max) max = p.value;
    }
    return { last: points[points.length - 1], min, max };
}

interface DomainInput {
    min: number;
    max: number;
    scale?: YScale;
    thresholds?: ThresholdBand[];
    references?: ReferenceMark[];
}

/**
 * Eje Y que contiene los datos, los umbrales y las referencias. Sin recorte por percentil:
 * en detección de fuga el pico es justo lo que se quiere ver.
 */
export function computeYDomain({ min, max, scale = {}, thresholds = [], references = [] }: DomainInput): [number, number] {
    let lo = min;
    let hi = max;
    for (const v of [...thresholds.map((t) => t.from), ...references.map((r) => r.value)]) {
        lo = Math.min(lo, v);
        hi = Math.max(hi, v);
    }
    if (!Number.isFinite(lo) || !Number.isFinite(hi)) return [scale.min ?? 0, scale.minTop ?? 1];

    // Un rango mínimo evita que el eje haga zoom sobre ruido: 0,06° de roll no debe verse como un escalón.
    if (scale.minSpan !== undefined && hi - lo < scale.minSpan) {
        const center = (hi + lo) / 2;
        lo = center - scale.minSpan / 2;
        hi = center + scale.minSpan / 2;
        if (min >= 0 && lo < 0) [lo, hi] = [0, scale.minSpan];
    }

    const range = hi - lo;
    const padding = range === 0 ? Math.abs(hi) * Y_PADDING_RATIO || 1 : range * Y_PADDING_RATIO;
    let lower = lo >= 0 ? Math.max(0, lo - padding) : lo - padding;
    let upper = hi + padding;

    if (scale.min !== undefined) lower = scale.min;
    if (scale.minTop !== undefined) upper = Math.max(upper, scale.minTop);
    if (upper <= lower) upper = lower + (padding || 1);

    // Bordes en múltiplos de un paso "redondo": el eje marca 0 · 150 · 300 y no termina en 514.
    const step = niceStep((upper - lower) / Y_TICK_INTERVALS);
    lower = scale.min ?? Math.floor(lower / step) * step;
    upper = Math.ceil(upper / step) * step;
    if (scale.max !== undefined) upper = Math.min(upper, scale.max);
    return [roundToStep(lower, step), roundToStep(upper, step)];
}

export const Y_TICK_INTERVALS = 4;

/** Menor paso de la serie 1 · 2 · 2,5 · 3 · 5 × 10ⁿ que sea ≥ raw. */
export function niceStep(raw: number): number {
    if (!(raw > 0)) return 1;
    const magnitude = 10 ** Math.floor(Math.log10(raw));
    const factor = [1, 2, 2.5, 3, 5, 10].find((f) => f * magnitude >= raw * (1 - 1e-9)) ?? 10;
    return factor * magnitude;
}

/** Quita el ruido de coma flotante (0,30000000000000004). */
function roundToStep(value: number, step: number): number {
    const decimals = Math.max(0, -Math.floor(Math.log10(step)) + 2);
    return Number(value.toFixed(decimals));
}

// Pasos "redondos" para el eje X; se elige el menor que no supere el número de marcas deseado.
const TICK_STEPS_MS = [HOUR_MS / 4, HOUR_MS / 2, HOUR_MS, 2 * HOUR_MS, 3 * HOUR_MS, 6 * HOUR_MS, 12 * HOUR_MS, DAY_MS, 2 * DAY_MS, 7 * DAY_MS];
const TARGET_TICKS = 7;

export interface TimeTicks {
    ticks: number[];
    stepMs: number;
}

/** Marcas del eje X alineadas a horas/días locales, para leer "las 06:00" y no "las 05:47". */
export function computeTimeTicks(start: number, end: number, targetTicks = TARGET_TICKS): TimeTicks {
    const span = end - start;
    const stepMs = TICK_STEPS_MS.find((s) => span / s <= targetTicks) ?? TICK_STEPS_MS[TICK_STEPS_MS.length - 1];

    // Se alinea sobre la medianoche local del inicio: con pasos de horas o días quedan en horas exactas.
    const anchor = new Date(start);
    anchor.setHours(0, 0, 0, 0);
    let t = anchor.getTime() + Math.ceil((start - anchor.getTime()) / stepMs) * stepMs;

    const ticks: number[] = [];
    for (; t <= end; t += stepMs) ticks.push(t);
    return { ticks, stepMs };
}

/** Las marcas de un día o más muestran la fecha; las de horas, la hora (y la fecha en cada medianoche). */
export function isDayTick(tick: number, stepMs: number): boolean {
    if (stepMs >= DAY_MS) return true;
    const d = new Date(tick);
    return d.getHours() === 0 && d.getMinutes() === 0;
}

// Si el punto más cercano está más lejos que esto, la otra gráfica no muestra tooltip (no hay dato comparable).
const SYNC_MAX_DISTANCE_MS = 90 * MINUTE_MS;

/**
 * Índice del valor más cercano a `target` en `values` (ascendentes), o -1 si no hay ninguno a menos
 * de SYNC_MAX_DISTANCE_MS. Sincroniza el cursor entre gráficas con cadencias distintas (15 min vs. 1 h).
 */
export function nearestIndex(values: readonly number[], target: number): number {
    if (values.length === 0) return -1;
    let lo = 0;
    let hi = values.length - 1;
    while (lo < hi) {
        const mid = (lo + hi) >> 1;
        if (values[mid] < target) lo = mid + 1;
        else hi = mid;
    }
    const best = lo > 0 && target - values[lo - 1] < values[lo] - target ? lo - 1 : lo;
    return Math.abs(values[best] - target) <= SYNC_MAX_DISTANCE_MS ? best : -1;
}

/** Marcas del eje Y en múltiplos de un paso redondo dentro del dominio. */
export function computeYTicks([lo, hi]: [number, number]): number[] {
    const step = niceStep((hi - lo) / Y_TICK_INTERVALS);
    const ticks: number[] = [];
    for (let v = Math.ceil(lo / step) * step; v <= hi + step * 1e-9; v += step) ticks.push(roundToStep(v, step));
    return ticks;
}
