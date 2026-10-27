import type { AlertSearchFilters } from "@/services/alertManagement";
import { EVENT_ORDER, LEVEL_ORDER, MANAGEMENT_STATUS_ORDER } from "../config/alertManagementDisplay";

// La búsqueda APLICADA vive en la URL: sobrevive a recargas y se comparte por enlace. Los valores múltiples
// van separados por comas y con los mismos códigos del backend (p. ej. ?nivel=NARANJA,ROJO&pagina=2).
export const HISTORY_PARAMS = {
    device: "dispositivo",
    from: "desde",
    to: "hasta",
    maxLevels: "nivel",
    events: "evento",
    statuses: "estado",
    page: "pagina",
} as const;

export const EMPTY_FILTERS: AlertSearchFilters = {
    device: null,
    from: null,
    to: null,
    maxLevels: [],
    events: [],
    statuses: [],
};

export interface HistorySearch {
    filters: AlertSearchFilters;
    page: number;
}

const FIRST_PAGE = 1;
const ISO_DAY = /^\d{4}-\d{2}-\d{2}$/;
const POSITIVE_INTEGER = /^[1-9]\d*$/;

/** true si es un día real en formato aaaa-mm-dd (rechaza p. ej. 2026-02-30). */
export function isIsoDay(value: string): boolean {
    if (!ISO_DAY.test(value)) return false;
    const date = new Date(`${value}T00:00:00Z`);
    return !Number.isNaN(date.getTime()) && date.toISOString().startsWith(value);
}

function parsePositiveInteger(raw: string | null): number | null {
    if (raw === null || !POSITIVE_INTEGER.test(raw)) return null;
    const value = Number(raw);
    return Number.isSafeInteger(value) ? value : null;
}

function parseDay(raw: string | null): string | null {
    return raw !== null && isIsoDay(raw) ? raw : null;
}

/** Solo valores conocidos, sin repetidos y en orden canónico: así la query key no cambia por el orden. */
function parseList<T extends string>(raw: string | null, allowed: readonly T[]): T[] {
    if (!raw) return [];
    const requested = new Set(raw.split(",").map((value) => value.trim().toUpperCase()));
    return allowed.filter((value) => requested.has(value));
}

/** Búsqueda aplicada desde la URL. Un valor manipulado o inválido se ignora en vez de romper la consulta. */
export function parseHistorySearch(params: URLSearchParams): HistorySearch {
    return {
        filters: {
            device: parsePositiveInteger(params.get(HISTORY_PARAMS.device)),
            from: parseDay(params.get(HISTORY_PARAMS.from)),
            to: parseDay(params.get(HISTORY_PARAMS.to)),
            maxLevels: parseList(params.get(HISTORY_PARAMS.maxLevels), LEVEL_ORDER),
            events: parseList(params.get(HISTORY_PARAMS.events), EVENT_ORDER),
            statuses: parseList(params.get(HISTORY_PARAMS.statuses), MANAGEMENT_STATUS_ORDER),
        },
        page: parsePositiveInteger(params.get(HISTORY_PARAMS.page)) ?? FIRST_PAGE,
    };
}

function setOrDelete(params: URLSearchParams, key: string, value: string | null) {
    if (value === null || value === "") params.delete(key);
    else params.set(key, value);
}

/**
 * Copia de los parámetros con la búsqueda indicada. Conserva los parámetros ajenos al historial
 * (p. ej. ?alerta= del modal) y omite la página 1 para que la URL quede limpia.
 */
export function withHistorySearch(previous: URLSearchParams, { filters, page }: HistorySearch): URLSearchParams {
    const next = new URLSearchParams(previous);
    setOrDelete(next, HISTORY_PARAMS.device, filters.device === null ? null : String(filters.device));
    setOrDelete(next, HISTORY_PARAMS.from, filters.from);
    setOrDelete(next, HISTORY_PARAMS.to, filters.to);
    setOrDelete(next, HISTORY_PARAMS.maxLevels, filters.maxLevels.join(","));
    setOrDelete(next, HISTORY_PARAMS.events, filters.events.join(","));
    setOrDelete(next, HISTORY_PARAMS.statuses, filters.statuses.join(","));
    setOrDelete(next, HISTORY_PARAMS.page, page > FIRST_PAGE ? String(page) : null);
    return next;
}

/** Representación canónica de la búsqueda; sirve para comparar sin depender de referencias. */
export function historySearchKey(search: HistorySearch): string {
    return withHistorySearch(new URLSearchParams(), search).toString();
}
