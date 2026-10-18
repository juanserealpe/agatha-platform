export type QuickRange = "1h" | "6h" | "1d" | "1w" | "1m";

export const QUICK_RANGE_OPTIONS: { value: QuickRange; label: string }[] = [
    { value: "1h", label: "1h" },
    { value: "6h", label: "6h" },
    { value: "1d", label: "1d" },
    { value: "1w", label: "1sem" },
    { value: "1m", label: "1mes" },
];

export const DEFAULT_RANGE: QuickRange = "1w"; // 7 días, según lo acordado con tu compañero

const ROUND_MS = 5 * 60_000;

export function getRangeDates(range: QuickRange): { startDate: string; endDate: string } {
    // Redondeo hacia arriba a 5 min: dentro de ese lapso la query es idéntica (hay caché)
    // y el rango siempre incluye la última lectura.
    const end = new Date(Math.floor(Date.now() / ROUND_MS) * ROUND_MS);
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