// Formatos que usa el backend en sus respuestas y en los textos del catálogo de mensajes. La demo los
// replica para que lo simulado se vea igual que lo real. Colombia no tiene horario de verano: UTC-5 fijo.

const COLOMBIA_OFFSET_MS = -5 * 60 * 60 * 1000;
const HOUR_MS = 60 * 60 * 1000;

/** Fecha corrida a hora de Colombia, para leer sus campos UTC como si fueran locales. */
function shiftToColombia(date: Date): Date {
    return new Date(date.getTime() + COLOMBIA_OFFSET_MS);
}

/** RFC 3339 en UTC sin milisegundos, como el stream: "2026-10-30T14:00:00Z". */
export function toUtcIso(date: Date): string {
    return date.toISOString().replace(/\.\d{3}Z$/, "Z");
}

/** RFC 3339 en hora de Colombia, como la API REST: "2026-08-24T11:00:00-05:00". */
export function toColombiaIso(date: Date): string {
    return shiftToColombia(date).toISOString().replace(/\.\d{3}Z$/, "-05:00");
}

/** Día de Colombia (aaaa-mm-dd), el formato de los filtros `from` y `to`. */
export function colombiaDay(date: Date): string {
    return shiftToColombia(date).toISOString().slice(0, 10);
}

/** Hora en lenguaje de campo, en hora de Colombia: "11:00 del 24/08" (messages.FormatHour). */
export function formatHour(date: Date): string {
    const iso = shiftToColombia(date).toISOString();
    return `${iso.slice(11, 16)} del ${iso.slice(8, 10)}/${iso.slice(5, 7)}`;
}

/** Índice o corte con coma: 2 decimales y un tercero solo si hace falta (messages.FormatValue). */
export function formatValue(value: number): string {
    return value.toFixed(3).replace(/0$/, "").replace(".", ",");
}

/** Inicio de la hora UTC (en Colombia coincide con la hora local: el desfase es entero). */
export function floorToHour(date: Date): Date {
    return new Date(Math.floor(date.getTime() / HOUR_MS) * HOUR_MS);
}

export function addHours(date: Date, hours: number): Date {
    return new Date(date.getTime() + hours * HOUR_MS);
}

/** Horas completas entre dos fechas. */
export function hoursBetween(from: Date, to: Date): number {
    return Math.floor((to.getTime() - from.getTime()) / HOUR_MS);
}
