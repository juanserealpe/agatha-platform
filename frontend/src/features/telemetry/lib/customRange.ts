/** Extremo del rango que el usuario está eligiendo en el calendario. */
export type RangeEdge = "start" | "end";

/** Rango mientras se edita: días locales a medianoche, cualquiera puede faltar. */
export interface RangeDraft {
    start?: Date;
    end?: Date;
}

export interface AppliedRange {
    startDate: string;
    endDate: string;
}

export const EMPTY_DRAFT: RangeDraft = {};

const MINUTE_MS = 60_000;

function startOfDay(date: Date): Date {
    return new Date(date.getFullYear(), date.getMonth(), date.getDate());
}

/**
 * Aplica el día elegido al extremo activo y dice cuál sigue.
 * Si el día dejaría el rango al revés, se reinterpreta en lugar de rechazarlo: así nunca hay que
 * "deshacer" una selección para corregirla.
 */
export function pickDay(draft: RangeDraft, edge: RangeEdge, picked: Date): { draft: RangeDraft; next: RangeEdge } {
    const day = startOfDay(picked);
    if (edge === "start") {
        const keepsEnd = draft.end && draft.end >= day;
        return { draft: { start: day, end: keepsEnd ? draft.end : undefined }, next: "end" };
    }
    if (draft.start && day < draft.start) {
        // Un fin anterior al inicio es en realidad un nuevo inicio; se sigue pidiendo el fin.
        return { draft: { start: day, end: undefined }, next: "end" };
    }
    return { draft: { start: draft.start, end: day }, next: draft.start ? "end" : "start" };
}

export function isComplete(draft: RangeDraft): draft is Required<RangeDraft> {
    return !!draft.start && !!draft.end;
}

/**
 * Rango para la consulta: desde el inicio del primer día hasta el final del último, pero nunca después
 * de "ahora" (el backend rechaza fechas futuras).
 */
export function toAppliedRange(draft: Required<RangeDraft>, now: number = Date.now()): AppliedRange {
    const endOfDay = new Date(draft.end);
    endOfDay.setHours(23, 59, 59, 999);
    const nowMinute = Math.floor(now / MINUTE_MS) * MINUTE_MS;
    const end = new Date(Math.min(endOfDay.getTime(), nowMinute));
    return { startDate: startOfDay(draft.start).toISOString(), endDate: end.toISOString() };
}

/** Rango ya aplicado → días del calendario, para reabrir el diálogo con lo que se está viendo. */
export function draftFromApplied(range: AppliedRange | null): RangeDraft {
    if (!range) return EMPTY_DRAFT;
    return { start: startOfDay(new Date(range.startDate)), end: startOfDay(new Date(range.endDate)) };
}
