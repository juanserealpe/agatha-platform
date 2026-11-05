import type { AlertEvent, AlertLevel, AlertSearchFilters, ManagementStatus } from "@/services/alertManagement";
import { isIsoDay } from "./historySearchParams";

/** Filtros mientras el usuario los edita. Las fechas son el texto tal cual lo escribe (dd/mm/aaaa). */
export interface FilterDraft {
    device: number | null;
    fromText: string;
    toText: string;
    maxLevels: AlertLevel[];
    events: AlertEvent[];
    statuses: ManagementStatus[];
}

export type DateField = "from" | "to";

export type DraftValidation =
    | { ok: true; filters: AlertSearchFilters }
    | { ok: false; field: DateField; message: string };

export const DATE_PLACEHOLDER = "dd/mm/aaaa";

const DISPLAY_DAY = /^(\d{1,2})\/(\d{1,2})\/(\d{4})$/;

const INVALID_DATE_MESSAGE: Record<DateField, string> = {
    from: "La fecha de inicio no es válida. Use el formato dd/mm/aaaa.",
    to: "La fecha fin no es válida. Use el formato dd/mm/aaaa.",
};

export const RANGE_ERROR_MESSAGE = "La fecha fin no puede ser anterior a la fecha de inicio.";

/** "24/08/2026" (o "4/8/2026", o "2026-08-24") → "2026-08-24"; null si no es un día real. */
export function parseDisplayDay(text: string): string | null {
    const value = text.trim();
    if (isIsoDay(value)) return value;

    const match = DISPLAY_DAY.exec(value);
    if (!match) return null;
    const [, day, month, year] = match;
    const iso = `${year}-${month.padStart(2, "0")}-${day.padStart(2, "0")}`;
    return isIsoDay(iso) ? iso : null;
}

/** "2026-08-24" → "24/08/2026". */
export function isoDayToDisplay(iso: string): string {
    const [year, month, day] = iso.split("-");
    return `${day}/${month}/${year}`;
}

/** Día elegido en el calendario (fecha local) → "aaaa-mm-dd". */
export function dateToIsoDay(date: Date): string {
    const month = String(date.getMonth() + 1).padStart(2, "0");
    const day = String(date.getDate()).padStart(2, "0");
    return `${date.getFullYear()}-${month}-${day}`;
}

/** "aaaa-mm-dd" → fecha local a medianoche, para marcar el día en el calendario. */
export function isoDayToDate(iso: string): Date {
    const [year, month, day] = iso.split("-").map(Number);
    return new Date(year, month - 1, day);
}

export function draftFromFilters(filters: AlertSearchFilters): FilterDraft {
    return {
        device: filters.device,
        fromText: filters.from ? isoDayToDisplay(filters.from) : "",
        toText: filters.to ? isoDayToDisplay(filters.to) : "",
        maxLevels: filters.maxLevels,
        events: filters.events,
        statuses: filters.statuses,
    };
}

function parseOptionalDay(text: string): string | null | undefined {
    if (text.trim() === "") return null;
    return parseDisplayDay(text) ?? undefined;
}

/**
 * Filtros listos para buscar, o el primer error a mostrar. Con una sola fecha el rango queda abierto;
 * "fecha fin" anterior a "fecha inicio" no se busca (el backend también lo rechaza con 400).
 */
export function validateDraft(draft: FilterDraft): DraftValidation {
    const from = parseOptionalDay(draft.fromText);
    if (from === undefined) return { ok: false, field: "from", message: INVALID_DATE_MESSAGE.from };

    const to = parseOptionalDay(draft.toText);
    if (to === undefined) return { ok: false, field: "to", message: INVALID_DATE_MESSAGE.to };

    // Las fechas aaaa-mm-dd se comparan como texto sin ambigüedad.
    if (from !== null && to !== null && to < from) return { ok: false, field: "to", message: RANGE_ERROR_MESSAGE };

    return {
        ok: true,
        filters: {
            device: draft.device,
            from,
            to,
            maxLevels: draft.maxLevels,
            events: draft.events,
            statuses: draft.statuses,
        },
    };
}
