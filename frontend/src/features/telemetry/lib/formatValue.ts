const LOCALE = "es-CO";

const bigFmt = new Intl.NumberFormat(LOCALE, { maximumFractionDigits: 0 });
const midFmt = new Intl.NumberFormat(LOCALE, { maximumFractionDigits: 2 });
const smallFmt = new Intl.NumberFormat(LOCALE, { maximumFractionDigits: 3 });

/** El backend manda "-" cuando la variable no tiene unidad. */
export function cleanUnit(unit: string | undefined): string {
    return !unit || unit === "-" ? "" : unit;
}

/** Decimales según la magnitud: 947 · 22,09 · 0,038. */
export function formatNumber(value: number): string {
    const abs = Math.abs(value);
    return (abs >= 100 ? bigFmt : abs >= 1 ? midFmt : smallFmt).format(value);
}

export function formatWithUnit(value: number, unit: string): string {
    const u = cleanUnit(unit);
    if (!u) return formatNumber(value);
    return u === "%" || u === "°" ? `${formatNumber(value)}${u}` : `${formatNumber(value)} ${u}`;
}

/** Decimales que necesita un paso del eje: 0,25 → 2 · 2,5 → 1 · 100 → 0. */
export function stepDecimals(step: number): number {
    const [, fraction = ""] = String(Number(step.toPrecision(6))).split(".");
    return fraction.length;
}

/**
 * Formateador de las marcas del eje Y con los decimales del paso, para que dos marcas
 * nunca se lean igual (0,9745 y 0,975 redondeadas ambas a "0,975").
 */
export function makeTickFormatter(step: number): (value: number) => string {
    const digits = stepDecimals(step);
    const fmt = new Intl.NumberFormat(LOCALE, { minimumFractionDigits: digits, maximumFractionDigits: digits });
    return (value) => fmt.format(value);
}
