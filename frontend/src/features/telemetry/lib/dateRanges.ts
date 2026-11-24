export type QuickRange = "1h" | "6h" | "1d" | "1w" | "1m";

interface QuickRangeOption {
    value: QuickRange;
    /** Texto del botón. */
    label: string;
    /** Valor en la URL (?rango=): corto y sin espacios. */
    param: string;
}

export const QUICK_RANGE_OPTIONS: readonly QuickRangeOption[] = [
    { value: "1h", label: "1 h", param: "1h" },
    { value: "6h", label: "6 h", param: "6h" },
    { value: "1d", label: "1 día", param: "1d" },
    { value: "1w", label: "1 sem", param: "1sem" },
    { value: "1m", label: "1 mes", param: "1mes" },
];

export const DEFAULT_RANGE: QuickRange = "1w"; // 7 días, según lo acordado con tu compañero

const ROUND_MS = 5 * 60_000;

export function getRangeDates(range: QuickRange, now: number = Date.now()): { startDate: string; endDate: string } {
    // Redondeo a 5 min: dentro de ese lapso la query es idéntica (hay caché) y el rango no se mueve en cada render.
    const end = new Date(Math.floor(now / ROUND_MS) * ROUND_MS);
    const start = new Date(end);
    switch (range) {
        case "1h": start.setHours(start.getHours() - 1); break;
        case "6h": start.setHours(start.getHours() - 6); break;
        case "1d": start.setDate(start.getDate() - 1); break;
        case "1w": start.setDate(start.getDate() - 7); break;
        case "1m": start.setMonth(start.getMonth() - 1); break;
    }
    return { startDate: start.toISOString(), endDate: end.toISOString() };
}
