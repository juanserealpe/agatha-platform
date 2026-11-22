import type { AlertLevel, DayRange, DeviceAlert } from "@/services/alertManagement";
import { formatColombiaIsoDay } from "@/shared/lib/formatDate";
import { LEVEL_ORDER } from "../config/alertManagementDisplay";

/**
 * Vista combinada (spec multiindicador A2, "Diseño del adaptador" en progreso_multiindicador.md): las alertas del
 * MISMO punto que se cruzan en el tiempo forman un grupo. El backend no las relaciona (A1): el grupo se calcula aquí
 * cada vez que se abre la vista y no se guarda.
 */

/** Lo que el grupo necesita de cada alerta. Lo cumplen `DeviceAlert` y `AlertDetail`. */
export type GroupCandidate = Pick<
    DeviceAlert,
    "id" | "code" | "event" | "state" | "started_at" | "ended_at" | "max_level" | "classification" | "levels"
>;

interface SpanMs {
    startMs: number;
    endMs: number;
}

/**
 * Tope de consultas para encontrar el grupo. Cada vuelta amplía el rango con lo encontrado; una cadena que siga
 * creciendo después de 5 vueltas se muestra con lo que haya (caso normal: 1 o 2).
 */
export const MAX_GROUP_ROUNDS = 5;

/**
 * La primera consulta pide estos días de más a cada lado de la alerta: así, en el caso normal, las alertas que se
 * cruzan con ella (y las encadenadas con esas) llegan en una sola consulta y no hace falta una segunda vuelta.
 */
export const FIRST_ROUND_PADDING_DAYS = 3;

const DAY_MS = 86_400_000;

function toMs(value: string): number {
    return new Date(value).getTime();
}

function isActive(alert: Pick<GroupCandidate, "state">): boolean {
    return alert.state === "ACTIVA";
}

/** Rango de la alerta: `[started_at, ended_at ?? ahora]`. */
export function alertSpanMs(alert: Pick<GroupCandidate, "state" | "started_at" | "ended_at">, now: number): SpanMs {
    const startMs = toMs(alert.started_at);
    const endMs = isActive(alert) || alert.ended_at === null ? now : toMs(alert.ended_at);
    return { startMs, endMs: Math.max(startMs, endMs) };
}

/** Cruce estricto, sin tolerancia: los regresos cercanos ya los cubre la reactivación del backend. */
export function spansOverlap(a: SpanMs, b: SpanMs): boolean {
    return a.startMs <= b.endMs && b.startMs <= a.endMs;
}

/**
 * Alertas encadenadas con `seed`: una entra si se cruza con CUALQUIER miembro (transitivo), así cada alerta queda en
 * un solo grupo y se llega al mismo grupo desde cualquiera de ellas. Ordenadas por inicio. Siempre incluye `seed`,
 * y su versión (el detalle abierto, más fresco) prevalece sobre la de `candidates`.
 */
export function connectedGroup<T extends GroupCandidate>(seed: T, candidates: readonly T[], now: number): T[] {
    const pool = [seed, ...candidates.filter((candidate) => candidate.id !== seed.id)];
    const spans = new Map(pool.map((alert) => [alert.id, alertSpanMs(alert, now)]));
    const members = [seed];
    const pending = pool.slice(1);
    for (let i = 0; i < members.length; i++) {
        const memberSpan = spans.get(members[i].id)!;
        for (let j = pending.length - 1; j >= 0; j--) {
            if (spansOverlap(memberSpan, spans.get(pending[j].id)!)) members.push(...pending.splice(j, 1));
        }
    }
    return members.sort((a, b) => spans.get(a.id)!.startMs - spans.get(b.id)!.startMs || a.id - b.id);
}

/** Del primer inicio al último fin del grupo. */
function groupSpanMs(members: readonly GroupCandidate[], now: number): SpanMs {
    const spans = members.map((member) => alertSpanMs(member, now));
    return { startMs: Math.min(...spans.map((s) => s.startMs)), endMs: Math.max(...spans.map((s) => s.endMs)) };
}

/** Días de Colombia que cubren el rango: el backend filtra por día completo (`to` incluido). */
export function spanToDays(span: SpanMs): DayRange {
    return { from: formatColombiaIsoDay(span.startMs) ?? "", to: formatColombiaIsoDay(span.endMs) ?? "" };
}

function coversDays(outer: DayRange, inner: DayRange): boolean {
    return inner.from >= outer.from && inner.to <= outer.to;
}

export interface AlertGroupResult<T extends GroupCandidate> {
    members: T[];
    /** false si se llegó al tope de vueltas y la cadena podía seguir. */
    complete: boolean;
}

/**
 * Busca el grupo de `seed`. Pide las alertas del punto que se cruzan con los días del grupo hasta ahora; si lo
 * encontrado estira el grupo fuera de esos días, vuelve a pedir con el rango ampliado. El backend filtra por día,
 * así que el cruce exacto (con horas) se vuelve a calcular aquí.
 */
export async function findAlertGroup<T extends GroupCandidate>(
    seed: T,
    fetchDays: (days: DayRange) => Promise<readonly T[]>,
    now: number,
    maxRounds: number = MAX_GROUP_ROUNDS,
    paddingDays: number = FIRST_ROUND_PADDING_DAYS
): Promise<AlertGroupResult<T>> {
    const seedSpan = alertSpanMs(seed, now);
    const padMs = paddingDays * DAY_MS;
    // El futuro no tiene alertas: el margen de la derecha no pasa de ahora.
    let days = spanToDays({ startMs: seedSpan.startMs - padMs, endMs: Math.min(seedSpan.endMs + padMs, now) });
    let members: T[] = [seed];
    for (let round = 0; round < maxRounds; round++) {
        members = connectedGroup(seed, await fetchDays(days), now);
        const needed = spanToDays(groupSpanMs(members, now));
        if (coversDays(days, needed)) return { members, complete: true };
        days = { from: needed.from < days.from ? needed.from : days.from, to: needed.to > days.to ? needed.to : days.to };
    }
    return { members, complete: false };
}

function higherLevel(a: AlertLevel, b: AlertLevel): AlertLevel {
    return LEVEL_ORDER.indexOf(b) > LEVEL_ORDER.indexOf(a) ? b : a;
}

/** Nivel en que está ahora una alerta activa: el de su tramo abierto (el endpoint del punto no trae `current_level`). */
export function memberCurrentLevel(member: Pick<GroupCandidate, "state" | "levels" | "max_level">): AlertLevel | null {
    if (!isActive(member)) return null;
    const open = member.levels.find((segment) => segment.ended_at === null);
    return open?.level ?? member.levels[member.levels.length - 1]?.level ?? member.max_level;
}

/** Tiempo en que alguna alerta del grupo estaba en alerta. Con una sola alerta es su duración (sin los huecos). */
function unionDurationMs(members: readonly GroupCandidate[], now: number): number {
    const intervals = members
        .flatMap((member) => {
            const alertEnd = alertSpanMs(member, now).endMs;
            return member.levels.map((segment) => ({
                startMs: toMs(segment.started_at),
                endMs: segment.ended_at ? toMs(segment.ended_at) : alertEnd,
            }));
        })
        .sort((a, b) => a.startMs - b.startMs);
    let total = 0;
    let cursor = -Infinity;
    for (const { startMs, endMs } of intervals) {
        const from = Math.max(startMs, cursor);
        if (endMs > from) total += endMs - from;
        cursor = Math.max(cursor, endMs);
    }
    return total;
}

export interface GroupSummary {
    size: number;
    /** Alguna alerta sigue activa. "Finalizada" solo cuando terminaron todas. */
    inProgress: boolean;
    /** En curso: el nivel ACTUAL más alto entre las activas. Finalizado: el máximo alcanzado (D4). */
    level: AlertLevel;
    /** Duración general (D6): tiempo cubierto por las alertas del grupo, sin contar dos veces lo que se cruza. */
    durationMs: number;
    /** Titila si titila alguna: activa, en Rojo y sin clasificar (A5). */
    blinking: boolean;
}

/** Valores generales del grupo. `members` nunca está vacío (siempre incluye la alerta abierta). */
export function summarizeGroup(members: readonly GroupCandidate[], now: number): GroupSummary {
    const active = members.filter(isActive);
    const currentLevels = active.map(memberCurrentLevel).filter((level): level is AlertLevel => level !== null);
    const level =
        currentLevels.length > 0
            ? currentLevels.reduce(higherLevel)
            : members.map((member) => member.max_level).reduce(higherLevel);
    return {
        size: members.length,
        inProgress: active.length > 0,
        level,
        durationMs: unionDurationMs(members, now),
        blinking: active.some((member) => memberCurrentLevel(member) === "ROJO" && member.classification === null),
    };
}
