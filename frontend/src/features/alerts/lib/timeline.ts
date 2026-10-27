import type { LevelSegment } from "@/services/alertManagement";

const MINUTE_MS = 60_000;
const HOUR_MINUTES = 60;
const DAY_HOURS = 24;
// Hasta 3 días se habla en horas, como los mensajes del backend ("lleva 29 h"); después, en días.
const HOURS_ONLY_LIMIT = 72;

function toMs(value: string): number {
    return new Date(value).getTime();
}

/** Duración de un tramo de color; el tramo abierto (sin fin) cuenta hasta `now`. */
export function segmentDurationMs(segment: Pick<LevelSegment, "started_at" | "ended_at">, now: number): number {
    const end = segment.ended_at ? toMs(segment.ended_at) : now;
    return Math.max(0, end - toMs(segment.started_at));
}

/** Duración acumulada del evento: suma de sus tramos (los huecos entre reactivaciones no cuentan). */
export function totalDurationMs(levels: readonly Pick<LevelSegment, "started_at" | "ended_at">[], now: number): number {
    return levels.reduce((sum, segment) => sum + segmentDurationMs(segment, now), 0);
}

/** "45 min", "3 h 30 min", "38 h", "3 d 8 h". Menos de un minuto: "< 1 min". */
export function formatDuration(ms: number): string {
    const totalMinutes = Math.floor(ms / MINUTE_MS);
    if (totalMinutes < 1) return "< 1 min";

    const totalHours = Math.floor(totalMinutes / HOUR_MINUTES);
    const minutes = totalMinutes % HOUR_MINUTES;

    if (totalHours >= HOURS_ONLY_LIMIT) {
        const days = Math.floor(totalHours / DAY_HOURS);
        const hours = totalHours % DAY_HOURS;
        return hours > 0 ? `${days} d ${hours} h` : `${days} d`;
    }
    if (totalHours > 0) return minutes > 0 ? `${totalHours} h ${minutes} min` : `${totalHours} h`;
    return `${minutes} min`;
}
