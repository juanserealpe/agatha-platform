import type { AlertEvent, AlertLevel, LevelOpenedBy, LevelSegment } from "@/services/alertManagement";
import { formatColombiaDayMonthTime, formatColombiaShort } from "@/shared/lib/formatDate";
import { OPENED_BY_LABEL } from "../config/alertExplanations";
import { levelStyle } from "../config/alertManagementDisplay";
import { formatDuration, totalDurationMs } from "./timeline";

/**
 * Franja de tiempo general del detalle (spec multiindicador §5): una barra por alerta sobre un eje común.
 * Recibe las alertas del grupo (vista combinada, lib/alertGroup.ts); con una alerta sola, una barra.
 */

export interface StripIndicatorInput {
    id: number;
    event: AlertEvent;
    levels: readonly Pick<LevelSegment, "seq" | "level" | "opened_by" | "started_at" | "ended_at">[];
    /** Fin de la alerta; null mientras sigue en curso. */
    endedAt: string | null;
    /** Código visible (ALR-…), para distinguir dos barras del mismo evento. */
    code?: string;
    /**
     * Es la alerta abierta en el modal: su fila se resalta y sus tramos navegan a la fase en la línea de tiempo.
     * Las demás del grupo se atenúan y un clic las abre. Por defecto true (una alerta sola).
     */
    current?: boolean;
}

/** `level`: un tramo de color de la alerta. `idle`: la alerta no estaba en alerta (gris, §5.1). */
export type StripPieceKind = "level" | "idle";

export interface StripPiece {
    kind: StripPieceKind;
    /** null en los tramos `idle`. */
    level: AlertLevel | null;
    /** `seq` del tramo de la API; null en los `idle`. Sirve para navegar a la fase (etapa 3). */
    seq: number | null;
    startMs: number;
    endMs: number;
    /** Posición y ancho en % del eje: proporcionales a la duración real (CA4). */
    offsetPct: number;
    widthPct: number;
    /** El tramo llega hasta "ahora" (alerta en curso). */
    reachesNow: boolean;
}

export interface StripMarker {
    /**
     * `change`: inicio de la barra o cambio de nivel. `now`: círculo hueco al final de una alerta en curso.
     * `end`: círculo relleno con el color de la última fase donde terminó una alerta finalizada.
     */
    kind: "change" | "now" | "end";
    atMs: number;
    offsetPct: number;
    /** Nivel al que se entra (relleno del círculo, §5.2), el actual en `now` o el último en `end`. */
    level: AlertLevel;
    /** Nivel anterior si el cambio es contiguo; null al abrir o al reactivarse tras un hueco. */
    from: AlertLevel | null;
    openedBy: LevelOpenedBy | null;
}

export interface StripBar {
    id: number;
    event: AlertEvent;
    code: string | null;
    current: boolean;
    /** Inicio del primer tramo y fin de la alerta (null si sigue en curso), para la fecha exacta de su fila. */
    startMs: number;
    endMs: number | null;
    /** Nivel del tramo abierto si sigue en curso; null si finalizó. */
    currentLevel: AlertLevel | null;
    pieces: StripPiece[];
    markers: StripMarker[];
    /** Duración de la alerta hasta ahora, como la "Duración general" de la línea de tiempo (sin los huecos). */
    durationMs: number;
}

export interface TimeStrip {
    /** Dominio visible del eje: de la primera a la última fecha de las alertas, con un margen a cada lado. */
    startMs: number;
    endMs: number;
    /** Alguna alerta sigue en curso: línea punteada de "Ahora". */
    active: boolean;
    /** "Ahora" y su posición en % del eje; null si todas finalizaron. */
    nowMs: number | null;
    nowOffsetPct: number | null;
    bars: StripBar[];
}

const FULL_PCT = 100;
/** Margen del eje a cada lado (3 % del rango), para que los círculos de los extremos y "Ahora" respiren. */
export const STRIP_PADDING_RATIO = 0.03;
// Margen de un eje sin duración (alerta recién abierta), para que no quede de ancho cero.
const MIN_PADDING_MS = 60_000;

export const IDLE_PIECE_LABEL = "Sin alerta";
export const NOW_LABEL = "Ahora";
// Mismo texto que el cierre de la línea de tiempo (A4): el fin no es un nivel "Normal".
export const END_LABEL = "Finalizada";
const IN_PROGRESS_RANGE_END = "en curso";

function toMs(value: string): number {
    return new Date(value).getTime();
}

function clamp(value: number, min: number, max: number): number {
    return Math.min(max, Math.max(min, value));
}

/** Arma la geometría de la franja. null si ninguna alerta tiene tramos todavía. */
export function buildTimeStrip(
    indicators: readonly StripIndicatorInput[],
    now: number,
    paddingRatio: number = STRIP_PADDING_RATIO
): TimeStrip | null {
    const withLevels = indicators.filter((indicator) => indicator.levels.length > 0);
    if (withLevels.length === 0) return null;

    const active = withLevels.some((indicator) => indicator.endedAt === null);
    const dataStartMs = Math.min(...withLevels.map((indicator) => Math.min(...indicator.levels.map((s) => toMs(s.started_at)))));
    const endCandidates = withLevels.map((indicator) => (indicator.endedAt === null ? now : toMs(indicator.endedAt)));
    const dataEndMs = Math.max(dataStartMs, active ? now : Math.max(...endCandidates));
    const paddingMs = paddingRatio > 0 ? Math.max(MIN_PADDING_MS, (dataEndMs - dataStartMs) * paddingRatio) : 0;
    const startMs = dataStartMs - paddingMs;
    const endMs = dataEndMs + paddingMs;
    // Un eje de duración cero (alerta recién abierta, sin margen) se dibuja igual: la barra ocupa todo el ancho.
    const spanMs = Math.max(1, endMs - startMs);
    const pct = (ms: number) => clamp(((ms - startMs) / spanMs) * FULL_PCT, 0, FULL_PCT);

    const bars = withLevels.map((indicator) => buildBar(indicator, { dataStartMs, dataEndMs, now, pct }));
    return { startMs, endMs, active, nowMs: active ? now : null, nowOffsetPct: active ? pct(now) : null, bars };
}

interface Axis {
    /** Primera y última fecha de las alertas (sin el margen): el gris de cada barra no entra al margen. */
    dataStartMs: number;
    dataEndMs: number;
    now: number;
    pct: (ms: number) => number;
}

function buildBar(indicator: StripIndicatorInput, axis: Axis): StripBar {
    const segments = [...indicator.levels].sort((a, b) => toMs(a.started_at) - toMs(b.started_at));
    const alertEndMs = indicator.endedAt === null ? axis.now : toMs(indicator.endedAt);
    const pieces: StripPiece[] = [];
    const markers: StripMarker[] = [];

    const pushPiece = (piece: Omit<StripPiece, "offsetPct" | "widthPct">) => {
        if (piece.endMs <= piece.startMs && pieces.length > 0) return;
        const offsetPct = axis.pct(piece.startMs);
        pieces.push({ ...piece, offsetPct, widthPct: Math.max(0, axis.pct(piece.endMs) - offsetPct) });
    };

    let cursor = axis.dataStartMs;
    let previous: { level: AlertLevel; endMs: number } | null = null;
    for (const segment of segments) {
        const segStart = toMs(segment.started_at);
        const segEnd = segment.ended_at ? toMs(segment.ended_at) : alertEndMs;
        if (segStart > cursor) {
            pushPiece({ kind: "idle", level: null, seq: null, startMs: cursor, endMs: segStart, reachesNow: false });
        }

        const contiguous = previous !== null && previous.endMs >= segStart;
        // Solo en cambios de nivel y al inicio de la barra o tras un hueco; nunca por cada hora (§5.2).
        if (!contiguous || previous?.level !== segment.level) {
            markers.push({
                kind: "change",
                atMs: segStart,
                offsetPct: axis.pct(segStart),
                level: segment.level,
                from: contiguous && previous ? previous.level : null,
                openedBy: segment.opened_by,
            });
        }

        const reachesNow = segment.ended_at === null && indicator.endedAt === null;
        pushPiece({ kind: "level", level: segment.level, seq: segment.seq, startMs: segStart, endMs: segEnd, reachesNow });
        cursor = Math.max(cursor, segEnd);
        previous = { level: segment.level, endMs: segEnd };
    }

    if (cursor < axis.dataEndMs) {
        pushPiece({ kind: "idle", level: null, seq: null, startMs: cursor, endMs: axis.dataEndMs, reachesNow: false });
    }

    const last = segments[segments.length - 1];
    if (indicator.endedAt === null && last && last.ended_at === null) {
        markers.push({ kind: "now", atMs: axis.now, offsetPct: axis.pct(axis.now), level: last.level, from: null, openedBy: null });
    } else if (indicator.endedAt !== null && last) {
        // Pedido del usuario (20-nov): la finalizada cierra su barra con un círculo, como "Ahora" en las activas.
        const endMs = last.ended_at ? toMs(last.ended_at) : alertEndMs;
        markers.push({ kind: "end", atMs: endMs, offsetPct: axis.pct(endMs), level: last.level, from: null, openedBy: null });
    }

    const active = indicator.endedAt === null;
    const openSegment = segments.find((segment) => segment.ended_at === null) ?? last;
    return {
        id: indicator.id,
        event: indicator.event,
        code: indicator.code ?? null,
        current: indicator.current ?? true,
        startMs: segments.length > 0 ? toMs(segments[0].started_at) : axis.dataStartMs,
        endMs: active ? null : alertEndMs,
        currentLevel: active && openSegment ? openSegment.level : null,
        pieces,
        markers,
        durationMs: totalDurationMs(segments, axis.now),
    };
}

function pieceLevelLabel(piece: Pick<StripPiece, "level">): string {
    return piece.level ? levelStyle(piece.level).label : IDLE_PIECE_LABEL;
}

function pieceEndLabel(piece: Pick<StripPiece, "endMs" | "reachesNow">): string {
    return piece.reachesNow ? NOW_LABEL.toLowerCase() : formatColombiaShort(piece.endMs);
}

/** Tooltip de un tramo: `24/08 22:00 – 25/08 19:00 · Rojo · 21 h`. */
export function pieceTooltip(piece: StripPiece): string {
    const range = `${formatColombiaShort(piece.startMs)} – ${pieceEndLabel(piece)}`;
    return `${range} · ${pieceLevelLabel(piece)} · ${formatDuration(piece.endMs - piece.startMs)}`;
}

/** Etiqueta accesible de un tramo (§10.3): `Anomalía acústica, Rojo, 24/08 22:00 a 25/08 19:00, 21 h`. */
export function pieceAccessibleLabel(piece: StripPiece, indicatorLabel: string): string {
    const range = `${formatColombiaShort(piece.startMs)} a ${pieceEndLabel(piece)}`;
    return `${indicatorLabel}, ${pieceLevelLabel(piece)}, ${range}, ${formatDuration(piece.endMs - piece.startMs)}`;
}

/**
 * Tooltip de un círculo (§5.2): `24/08 22:00 · Naranja → Rojo`, `24/08 05:00 · Apertura · Amarillo`,
 * `Ahora · Rojo` o `25/08 19:00 · Finalizada · Rojo`.
 */
export function markerTooltip(marker: StripMarker): string {
    const level = levelStyle(marker.level).label;
    if (marker.kind === "now") return `${NOW_LABEL} · ${level}`;
    const at = formatColombiaShort(marker.atMs);
    if (marker.kind === "end") return `${at} · ${END_LABEL} · ${level}`;
    if (marker.from) return `${at} · ${levelStyle(marker.from).label} → ${level}`;
    const reason = marker.openedBy ? OPENED_BY_LABEL[marker.openedBy] : null;
    return reason ? `${at} · ${reason} · ${level}` : `${at} · ${level}`;
}

/** Fechas exactas de la alerta en su fila: "18 nov 16:00 → en curso" o "19 nov 16:00 → 20 nov 16:00". */
export function barRangeLabel(bar: Pick<StripBar, "startMs" | "endMs">): string {
    const end = bar.endMs === null ? IN_PROGRESS_RANGE_END : formatColombiaDayMonthTime(bar.endMs);
    return `${formatColombiaDayMonthTime(bar.startMs)} → ${end}`;
}

/** Resumen accesible de una fila: "Anomalía acústica ALR-000120, de 18 nov 16:00 a en curso, nivel actual Naranja". */
export function barAccessibleLabel(bar: Pick<StripBar, "startMs" | "endMs" | "currentLevel">, name: string): string {
    const end = bar.endMs === null ? IN_PROGRESS_RANGE_END : formatColombiaDayMonthTime(bar.endMs);
    const state = bar.currentLevel ? `nivel actual ${levelStyle(bar.currentLevel).label}` : END_LABEL.toLowerCase();
    return `${name}, de ${formatColombiaDayMonthTime(bar.startMs)} a ${end}, ${state}`;
}

/** Rango bajo el cursor en la línea de tiempo (§5.3). `endMs` null = hasta ahora; igual a `startMs` = un instante. */
export interface HoverRange {
    startMs: number;
    endMs: number | null;
}

/** Banda punteada del cursor de tiempo sobre todas las barras (§5.3). */
export interface StripCursor {
    offsetPct: number;
    /** 0 en un instante (un mensaje): se dibuja como una línea. */
    widthPct: number;
}

/**
 * Posición del cursor de tiempo. Sin etiqueta de texto (decisión del usuario, 20-nov): tapaba la franja y la
 * fecha ya está en la fase o el mensaje bajo el cursor.
 */
export function buildStripCursor(strip: TimeStrip, range: HoverRange): StripCursor {
    const startMs = clamp(range.startMs, strip.startMs, strip.endMs);
    const endMs = clamp(range.endMs ?? strip.endMs, startMs, strip.endMs);
    const spanMs = Math.max(1, strip.endMs - strip.startMs);
    const pct = (value: number) => ((value - strip.startMs) / spanMs) * FULL_PCT;
    return { offsetPct: pct(startMs), widthPct: pct(endMs) - pct(startMs) };
}

// Franja compacta (§5.3, ≈54 px con una barra): sin el eje de fechas; cada barra en una fila de 16 px con su
// nombre, duración y círculos; 4 px entre filas, 10 px de relleno arriba y abajo (más holgada, pedido del
// usuario), 1 px de borde y 8 px de aire arriba y abajo de la franja pegada. TimeStrip
// usa este alto y la línea de tiempo lo usa (más un respiro) como scroll-margin-top, para que la franja no
// tape el título de la fase.
const COMPACT_ROW_PX = 16;
const COMPACT_ROW_GAP_PX = 4;
const COMPACT_CHROME_PX = 2 * 10 + 2 * 1 + 2 * 8;

export function compactStripHeightPx(barCount: number): number {
    const rows = Math.max(1, barCount);
    return COMPACT_CHROME_PX + rows * COMPACT_ROW_PX + (rows - 1) * COMPACT_ROW_GAP_PX;
}
