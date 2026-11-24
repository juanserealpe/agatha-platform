import { DEFAULT_RANGE, QUICK_RANGE_OPTIONS, type QuickRange } from "./dateRanges";
import { toAppliedRange, type AppliedRange } from "./customRange";

// El rango de las gráficas vive en la URL: sobrevive a recargas, se comparte por enlace y es el mismo en todas las
// pestañas. Va en español porque el usuario lo ve.
const RANGE_PARAM = "rango";
const FROM_PARAM = "desde";
const TO_PARAM = "hasta";

/** Rango elegido: uno rápido (avanza con el reloj) o días fijos del calendario. */
export type RangeSelection = { kind: "quick"; range: QuickRange } | { kind: "custom"; start: Date; end: Date };

const DEFAULT_SELECTION: RangeSelection = { kind: "quick", range: DEFAULT_RANGE };
const DAY_PATTERN = /^(\d{4})-(\d{2})-(\d{2})$/;

/** "2026-11-24" → medianoche local de ese día, o null si no es una fecha real. */
function parseDay(raw: string | null): Date | null {
    const match = raw ? DAY_PATTERN.exec(raw) : null;
    if (!match) return null;
    const [year, month, day] = match.slice(1).map(Number);
    const date = new Date(year, month - 1, day);
    // new Date corrige fechas imposibles (31 de febrero → marzo): si cambió, no era válida.
    return date.getMonth() === month - 1 && date.getDate() === day ? date : null;
}

function formatDay(date: Date): string {
    const pad = (n: number) => String(n).padStart(2, "0");
    return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
}

/** Lee el rango de la URL. Un valor inválido, al revés o en el futuro cae al rango por defecto. */
export function parseRangeSelection(params: URLSearchParams, now: number = Date.now()): RangeSelection {
    const start = parseDay(params.get(FROM_PARAM));
    const end = parseDay(params.get(TO_PARAM));
    if (start && end && start <= end && start.getTime() <= now) return { kind: "custom", start, end };

    const param = params.get(RANGE_PARAM);
    const quick = QUICK_RANGE_OPTIONS.find((option) => option.param === param);
    return quick ? { kind: "quick", range: quick.value } : DEFAULT_SELECTION;
}

/** Escribe el rango en la URL sin tocar los demás parámetros (?alerta=, filtros). El rango por defecto no se escribe. */
export function withRangeSelection(params: URLSearchParams, selection: RangeSelection): URLSearchParams {
    const next = new URLSearchParams(params);
    next.delete(RANGE_PARAM);
    next.delete(FROM_PARAM);
    next.delete(TO_PARAM);
    if (selection.kind === "custom") {
        next.set(FROM_PARAM, formatDay(selection.start));
        next.set(TO_PARAM, formatDay(selection.end));
    } else if (selection.range !== DEFAULT_RANGE) {
        const param = QUICK_RANGE_OPTIONS.find((option) => option.value === selection.range)?.param;
        if (param) next.set(RANGE_PARAM, param);
    }
    return next;
}

/** Rango aplicado (ISO) → días del calendario que lo contienen. */
export function customSelectionFrom(range: AppliedRange): RangeSelection {
    const toDay = (iso: string) => {
        const date = new Date(iso);
        return new Date(date.getFullYear(), date.getMonth(), date.getDate());
    };
    return { kind: "custom", start: toDay(range.startDate), end: toDay(range.endDate) };
}

/** Días del calendario → ventana de la consulta (hasta el final del último día, sin pasar de ahora). */
export function customWindow(selection: Extract<RangeSelection, { kind: "custom" }>, now: number = Date.now()): AppliedRange {
    return toAppliedRange({ start: selection.start, end: selection.end }, now);
}
