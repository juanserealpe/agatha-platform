import type { AlertEvent, AlertLevel, LevelOpenedBy, LevelSegment } from "@/services/alertManagement";
import { formatColombiaShort } from "@/shared/lib/formatDate";
import { OPENED_BY_LABEL } from "../config/alertExplanations";
import { levelStyle } from "../config/alertManagementDisplay";
import { formatDuration, totalDurationMs } from "./timeline";

/**
 * Franja de tiempo general del detalle (spec multiindicador §5): una barra por alerta sobre un eje común.
 * Hoy la vista recibe una sola alerta; el adaptador de la etapa 4 le pasará las del grupo sin cambiar esto.
 */

export interface StripIndicatorInput {
    id: number;
    event: AlertEvent;
    levels: readonly Pick<LevelSegment, "seq" | "level" | "opened_by" | "started_at" | "ended_at">[];
    /** Fin de la alerta; null mientras sigue en curso. */
    endedAt: string | null;
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
    /** `change`: inicio de la barra o cambio de nivel. `now`: círculo hueco al final de una alerta en curso. */
    kind: "change" | "now";
    atMs: number;
    offsetPct: number;
    /** Nivel al que se entra (relleno del círculo, §5.2) o el actual en `now`. */
    level: AlertLevel;
    /** Nivel anterior si el cambio es contiguo; null al abrir o al reactivarse tras un hueco. */
    from: AlertLevel | null;
    openedBy: LevelOpenedBy | null;
}

export interface StripBar {
    id: number;
    event: AlertEvent;
    pieces: StripPiece[];
    markers: StripMarker[];
    /** Duración de la alerta hasta ahora, como la "Duración general" de la línea de tiempo (sin los huecos). */
    durationMs: number;
}

/** Marca del eje: solo donde hay un círculo (cambio de nivel o "Ahora"), para que cada fecha tenga a qué referirse. */
export interface StripTick {
    atMs: number;
    offsetPct: number;
    isNow: boolean;
}

export interface TimeStrip {
    startMs: number;
    endMs: number;
    /** Alguna alerta sigue en curso: el eje termina en "Ahora". */
    active: boolean;
    bars: StripBar[];
    ticks: StripTick[];
}

const FULL_PCT = 100;

export const IDLE_PIECE_LABEL = "Sin alerta";
export const NOW_LABEL = "Ahora";

function toMs(value: string): number {
    return new Date(value).getTime();
}

function clamp(value: number, min: number, max: number): number {
    return Math.min(max, Math.max(min, value));
}

/** Arma la geometría de la franja. null si ninguna alerta tiene tramos todavía. */
export function buildTimeStrip(indicators: readonly StripIndicatorInput[], now: number): TimeStrip | null {
    const withLevels = indicators.filter((indicator) => indicator.levels.length > 0);
    if (withLevels.length === 0) return null;

    const active = withLevels.some((indicator) => indicator.endedAt === null);
    const startMs = Math.min(...withLevels.map((indicator) => Math.min(...indicator.levels.map((s) => toMs(s.started_at)))));
    const endCandidates = withLevels.map((indicator) => (indicator.endedAt === null ? now : toMs(indicator.endedAt)));
    const endMs = Math.max(startMs, active ? now : Math.max(...endCandidates));
    // Un eje de duración cero (alerta recién abierta) se dibuja igual: la barra ocupa todo el ancho.
    const spanMs = Math.max(1, endMs - startMs);
    const pct = (ms: number) => clamp(((ms - startMs) / spanMs) * FULL_PCT, 0, FULL_PCT);

    const bars = withLevels.map((indicator) => buildBar(indicator, { startMs, endMs, now, pct }));
    return { startMs, endMs, active, bars, ticks: buildTicks(bars) };
}

/** Una marca por instante con círculo en alguna barra; varias barras que cambian a la vez comparten la marca. */
function buildTicks(bars: readonly StripBar[]): StripTick[] {
    const byTime = new Map<number, StripTick>();
    for (const marker of bars.flatMap((bar) => bar.markers)) {
        const current = byTime.get(marker.atMs);
        const isNow = marker.kind === "now" || (current?.isNow ?? false);
        byTime.set(marker.atMs, { atMs: marker.atMs, offsetPct: marker.offsetPct, isNow });
    }
    return [...byTime.values()].sort((a, b) => a.atMs - b.atMs);
}

export interface PlacedTick {
    tick: StripTick;
    /** Borde izquierdo de la etiqueta, dentro de la pista. */
    leftPx: number;
    widthPx: number;
    /** Posición del círculo al que apunta. */
    centerPx: number;
}

/**
 * Coloca las etiquetas del eje lo más cerca posible de su círculo sin solaparse ni salirse de la pista
 * (§5.1). Si varios cambios caen muy juntos, las etiquetas se corren a los lados (la línea guía las une con
 * su círculo) en vez de ocultarse. Solo si no caben todas se quitan las intermedias más apretadas; la
 * primera y "Ahora" se conservan siempre.
 */
export function placeAxisLabels(
    ticks: readonly StripTick[],
    trackWidthPx: number,
    labelWidthPx: (tick: StripTick) => number,
    gapPx: number
): PlacedTick[] {
    if (trackWidthPx <= 0) return [];
    const items = ticks.map((tick) => ({
        tick,
        widthPx: labelWidthPx(tick),
        centerPx: (tick.offsetPct / FULL_PCT) * trackWidthPx,
    }));

    const needed = () => items.reduce((sum, item) => sum + item.widthPx, 0) + gapPx * Math.max(0, items.length - 1);
    while (items.length > 2 && needed() > trackWidthPx) {
        // La intermedia más pegada a su vecina anterior (nunca la primera ni "Ahora").
        let drop = -1;
        for (let i = 1; i < items.length; i++) {
            if (items[i].tick.isNow) continue;
            if (drop === -1 || items[i].centerPx - items[i - 1].centerPx < items[drop].centerPx - items[drop - 1].centerPx) drop = i;
        }
        if (drop === -1) break;
        items.splice(drop, 1);
    }

    // Ida: cada etiqueta centrada en su círculo o empujada a la derecha de la anterior.
    const lefts = items.map((item) => clamp(item.centerPx - item.widthPx / 2, 0, Math.max(0, trackWidthPx - item.widthPx)));
    for (let i = 1; i < items.length; i++) {
        lefts[i] = Math.max(lefts[i], lefts[i - 1] + items[i - 1].widthPx + gapPx);
    }
    // Vuelta: las que se salieron por la derecha retroceden, empujando a las anteriores.
    for (let i = items.length - 1; i >= 0; i--) {
        const limit = i === items.length - 1 ? trackWidthPx - items[i].widthPx : lefts[i + 1] - gapPx - items[i].widthPx;
        lefts[i] = Math.max(0, Math.min(lefts[i], limit));
    }

    return items.map((item, i) => ({ ...item, leftPx: lefts[i] }));
}

interface Axis {
    startMs: number;
    endMs: number;
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

    let cursor = axis.startMs;
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

    if (cursor < axis.endMs) {
        pushPiece({ kind: "idle", level: null, seq: null, startMs: cursor, endMs: axis.endMs, reachesNow: false });
    }

    const last = segments[segments.length - 1];
    if (indicator.endedAt === null && last && last.ended_at === null) {
        markers.push({ kind: "now", atMs: axis.now, offsetPct: axis.pct(axis.now), level: last.level, from: null, openedBy: null });
    }

    return { id: indicator.id, event: indicator.event, pieces, markers, durationMs: totalDurationMs(segments, axis.now) };
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

/** Tooltip de un círculo (§5.2): `24/08 22:00 · Naranja → Rojo`, `24/08 05:00 · Apertura · Amarillo` o `Ahora · Rojo`. */
export function markerTooltip(marker: StripMarker): string {
    const level = levelStyle(marker.level).label;
    if (marker.kind === "now") return `${NOW_LABEL} · ${level}`;
    const at = formatColombiaShort(marker.atMs);
    if (marker.from) return `${at} · ${levelStyle(marker.from).label} → ${level}`;
    const reason = marker.openedBy ? OPENED_BY_LABEL[marker.openedBy] : null;
    return reason ? `${at} · ${reason} · ${level}` : `${at} · ${level}`;
}

/** Texto de una marca del eje. */
export function tickLabel(tick: StripTick): string {
    return tick.isNow ? NOW_LABEL : formatColombiaShort(tick.atMs);
}

/** La marca del eje en renglones: fecha arriba y hora abajo, para que ocupe la mitad del ancho. */
export function tickLabelLines(tick: StripTick): string[] {
    return tickLabel(tick).split(" ");
}
