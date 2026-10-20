import type { LeakThresholds } from "@/services/dashboard/series.types";
import { LEAK_LEVEL_STYLES, type LeakLevel } from "../config/leakThresholds";
import type { ThresholdBand } from "../config/seriesSections";

/**
 * Nivel que marca el umbral para un valor de I. Es el `raw_level` del Rule Engine: no aplica
 * persistencia ni histéresis, así que una hora puede marcar amarillo sin que exista alerta.
 */
export function leakLevelOf(value: number, t: LeakThresholds): LeakLevel {
    if (value >= t.rojo) return "rojo";
    if (value >= t.naranja) return "naranja";
    if (value >= t.amarillo) return "amarillo";
    return "normal";
}

const thresholdFmt = new Intl.NumberFormat("es-CO", { maximumFractionDigits: 2 });

function band(level: Exclude<LeakLevel, "normal">, from: number, to?: number): ThresholdBand {
    const { label, hex } = LEAK_LEVEL_STYLES[level];
    return { label: `${label} ≥ ${thresholdFmt.format(from)}`, from, to, color: hex };
}

export function leakThresholdBands(t: LeakThresholds): ThresholdBand[] {
    return [band("amarillo", t.amarillo, t.naranja), band("naranja", t.naranja, t.rojo), band("rojo", t.rojo)];
}
