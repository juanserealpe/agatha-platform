const LOCALE = "es-CO";

// Por debajo de este umbral se muestra tiempo relativo ("hace 5 min"); por encima, fecha absoluta.
const RECENT_THRESHOLD_MS = 24 * 60 * 60 * 1000;
const MINUTE_MS = 60 * 1000;
const HOUR_MS = 60 * MINUTE_MS;

const EMPTY_VALUE = "—";

export type DateInput = string | number | Date;

const fullFormatter = new Intl.DateTimeFormat(LOCALE, {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hourCycle: "h23",
});

const shortFormatter = new Intl.DateTimeFormat(LOCALE, {
    day: "2-digit",
    month: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hourCycle: "h23",
});

const clockFormatter = new Intl.DateTimeFormat(LOCALE, { hour: "2-digit", minute: "2-digit", hourCycle: "h23" });

const dayMonthFormatter = new Intl.DateTimeFormat(LOCALE, { day: "numeric", month: "short" });

const relativeFormatter =new Intl.RelativeTimeFormat(LOCALE, { numeric: "auto", style: "short" });

// Las fechas del backend se muestran siempre en hora de Colombia, sin depender de la zona del navegador.
const COLOMBIA_TIME_ZONE = "America/Bogota";

const colombiaDateTimeFormatter = new Intl.DateTimeFormat(LOCALE, {
    timeZone: COLOMBIA_TIME_ZONE,
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    hourCycle: "h23",
});

type DatePart = "day" | "month" | "year" | "hour" | "minute";

function colombiaParts(date: Date): Record<DatePart, string> {
    const parts = colombiaDateTimeFormatter.formatToParts(date);
    const part = (type: DatePart) => parts.find((p) => p.type === type)?.value ?? "";
    return { day: part("day"), month: part("month"), year: part("year"), hour: part("hour"), minute: part("minute") };
}

function toDate(value: DateInput): Date | null {
    const date = value instanceof Date ? value : new Date(value);
    return Number.isNaN(date.getTime()) ? null : date;
}

/** Valor para el atributo `dateTime` de <time>. */
export function toIsoString(value: DateInput): string | undefined {
    return toDate(value)?.toISOString();
}

/** Fecha y hora completas en 24 h, p. ej. "11/10/2026, 21:07:15". Pensada para tooltips. */
export function formatAbsolute(value: DateInput): string {
    const date = toDate(value);
    return date ? fullFormatter.format(date) : EMPTY_VALUE;
}

/** Fecha y hora cortas en 24 h, siempre absolutas, p. ej. "11/10 21:07". */
export function formatShort(value: DateInput): string {
    const date = toDate(value);
    return date ? shortFormatter.format(date) : EMPTY_VALUE;
}

/** Solo la hora en 24 h, p. ej. "21:07". Pensada para ejes de gráficas. */
export function formatClock(value: DateInput): string {
    const date = toDate(value);
    return date ? clockFormatter.format(date) : EMPTY_VALUE;
}

/** Día y mes abreviado, p. ej. "11 oct". Pensada para ejes de gráficas. */
export function formatDayMonth(value: DateInput): string {
    const date = toDate(value);
    if (!date) return EMPTY_VALUE;
    // es-CO formatea "11 de oct."; en un eje sobra el conector, así que se arma con las partes.
    const parts = dayMonthFormatter.formatToParts(date);
    const part = (type: Intl.DateTimeFormatPartTypes) => parts.find((p) => p.type === type)?.value ?? "";
    return `${part("day")} ${part("month").replace(".", "")}`;
}

/** Tiempo transcurrido, p. ej. "hace 5 min" o "hace 3 h". */
export function formatRelative(value: DateInput, now: number = Date.now()): string {
    const date = toDate(value);
    if (!date) return EMPTY_VALUE;

    const elapsedMs = Math.max(0, now - date.getTime());
    if (elapsedMs < MINUTE_MS) return "hace instantes";
    if (elapsedMs < HOUR_MS) return relativeFormatter.format(-Math.floor(elapsedMs / MINUTE_MS), "minute");
    return relativeFormatter.format(-Math.floor(elapsedMs / HOUR_MS), "hour");
}

/** Relativo si es reciente; si no, fecha corta en 24 h, p. ej. "11/10 21:07". */
export function formatTimestamp(value: DateInput, now: number = Date.now()): string {
    const date = toDate(value);
    if (!date) return EMPTY_VALUE;
    return now - date.getTime() < RECENT_THRESHOLD_MS
        ? formatRelative(date, now)
        : shortFormatter.format(date);
}

/** Fecha y hora en hora de Colombia, 24 h, p. ej. "24/08/2026 11:00". */
export function formatColombiaDateTime(value: DateInput): string {
    const date = toDate(value);
    if (!date) return EMPTY_VALUE;
    const { day, month, year, hour, minute } = colombiaParts(date);
    return `${day}/${month}/${year} ${hour}:${minute}`;
}

/** Solo la fecha en hora de Colombia, p. ej. "24/08/2026". */
export function formatColombiaDate(value: DateInput): string {
    const date = toDate(value);
    if (!date) return EMPTY_VALUE;
    const { day, month, year } = colombiaParts(date);
    return `${day}/${month}/${year}`;
}

/** Marca de tiempo para nombres de archivo en hora de Colombia, p. ej. "20261026_1504". */
export function formatColombiaFileStamp(value: DateInput): string {
    const date = toDate(value);
    if (!date) return "";
    const { day, month, year, hour, minute } = colombiaParts(date);
    return `${year}${month}${day}_${hour}${minute}`;
}
