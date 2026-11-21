import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import { EVENT_ICON, EVENT_LABEL, levelStyle } from "../../config/alertManagementDisplay";
import type { PhaseRef } from "../../hooks/useTimelineNavigation";
import type { AxisTick } from "../../lib/timeAxis";
import { formatDuration } from "../../lib/timeline";
import {
    barAccessibleLabel,
    barRangeLabel,
    markerTooltip,
    pieceAccessibleLabel,
    pieceTooltip,
    type StripBar,
    type StripMarker,
    type StripPiece,
} from "../../lib/timeStrip";
import { TimeStripGrid } from "./TimeStripAxis";
import {
    COMPACT_ROW_GRID_CLASS,
    CURRENT_ROW_CLASS,
    DIMMED_TRACK_CLASS,
    edgeRounding,
    EXPANDED_ROW_GRID_CLASS,
    FOCUS_CLASS,
    levelCutClass,
    pieceColorClass,
    piecePosition,
    ROW_PAD_CLASS,
    SELECTED_PIECE_CLASS,
    TRACK_INSET_CLASS,
} from "./timeStripStyles";

/** "Anomalía acústica" o, si la barra trae código (varias alertas), "Anomalía acústica ALR-000120". */
function barName(bar: StripBar): string {
    return bar.code ? `${EVENT_LABEL[bar.event]} ${bar.code}` : EVENT_LABEL[bar.event];
}

function Hint({ text, children }: { text: string; children: React.ReactElement }) {
    return (
        <Tooltip>
            <TooltipTrigger render={children} />
            <TooltipContent>{text}</TooltipContent>
        </Tooltip>
    );
}

/** Lo que pasa al hacer clic en una barra: navegar a la fase (alerta abierta) o abrir otra alerta del grupo. */
export interface BarActions {
    selected: PhaseRef | null;
    onSelect: (target: PhaseRef) => void;
    onOpenAlert: (alertId: number) => void;
    /** Al acercarse a otra alerta del grupo: precargar su detalle para que abra sin espera. */
    onPrefetchAlert: (alertId: number) => void;
}

interface PieceProps {
    piece: StripPiece;
    bar: StripBar;
    className: string;
    actions: BarActions;
}

function Piece({ piece, bar, className, actions }: PieceProps) {
    const label = pieceAccessibleLabel(piece, barName(bar));
    const base = cn("absolute inset-y-0 min-w-0.5", FOCUS_CLASS, pieceColorClass(piece), className);
    const { seq } = piece;
    // Los tramos grises no navegan: solo informan (CA7).
    if (piece.kind === "idle" || seq === null) {
        return (
            <Hint text={pieceTooltip(piece)}>
                <span tabIndex={0} role="img" aria-label={label} className={base} style={piecePosition(piece)} />
            </Hint>
        );
    }
    // Otra alerta del grupo: sus fases no están en pantalla; el clic abre esa alerta.
    if (!bar.current) {
        return (
            <Hint text={`${pieceTooltip(piece)} · Clic para abrir ${bar.code ?? "esta alerta"}`}>
                <button
                    type="button"
                    aria-label={`${label}. Abrir esta alerta`}
                    onClick={() => actions.onOpenAlert(bar.id)}
                    className={cn(base, "cursor-pointer hover:brightness-95")}
                    style={piecePosition(piece)}
                />
            </Hint>
        );
    }
    const selected = actions.selected?.alertId === bar.id && actions.selected.seq === seq;
    return (
        <Hint text={pieceTooltip(piece)}>
            <button
                type="button"
                aria-label={`${label}. Ir a esta fase`}
                aria-pressed={selected}
                onClick={() => actions.onSelect({ alertId: bar.id, seq })}
                className={cn(base, "cursor-pointer hover:brightness-95", selected && SELECTED_PIECE_CLASS)}
                style={piecePosition(piece)}
            />
        </Hint>
    );
}

function Marker({ marker }: { marker: StripMarker }) {
    const style = levelStyle(marker.level);
    const text = markerTooltip(marker);
    return (
        <Hint text={text}>
            <span
                tabIndex={0}
                role="img"
                aria-label={text}
                className={cn(
                    "absolute top-1/2 z-20 size-2.5 -translate-x-1/2 -translate-y-1/2 rounded-full",
                    FOCUS_CLASS,
                    // §5.2: relleno del nivel al que se entra con borde del fondo del panel; "Ahora" va hueco.
                    marker.kind === "now" ? cn("border-2 bg-white", style.borderClass) : cn("ring-2 ring-white", style.dotClass)
                )}
                style={{ left: `${marker.offsetPct}%` }}
            />
        </Hint>
    );
}

/** Líneas del eje y "Ahora" para las barras (las mismas en todas las filas). */
export interface StripGridLines {
    ticks: readonly AxisTick[];
    nowOffsetPct: number | null;
}

interface TrackProps {
    bar: StripBar;
    dimmed: boolean;
    cutColorClass: string;
    grid: StripGridLines;
    actions: BarActions;
}

function Track({ bar, dimmed, cutColorClass, grid, actions }: TrackProps) {
    return (
        <div className={cn(TRACK_INSET_CLASS, dimmed && DIMMED_TRACK_CLASS)}>
            <div className="relative h-2.5">
                {/* Celular: líneas cortas alrededor de la barra, sin cruzar el nombre ni las fechas de la fila. */}
                <div aria-hidden className="pointer-events-none absolute -inset-y-1.5 inset-x-0 sm:hidden">
                    <TimeStripGrid {...grid} />
                </div>
                {bar.pieces.map((piece, index) => (
                    <Piece
                        key={`${piece.kind}-${piece.startMs}`}
                        piece={piece}
                        bar={bar}
                        className={cn(edgeRounding(index, bar.pieces.length), levelCutClass(bar.pieces, index, cutColorClass))}
                        actions={actions}
                    />
                ))}
                {bar.markers.map((marker) => (
                    <Marker key={`${marker.kind}-${marker.atMs}`} marker={marker} />
                ))}
            </div>
        </div>
    );
}

interface BarProps {
    bar: StripBar;
    /** Hay varias alertas en la franja: se resalta la abierta y se atenúan las demás. */
    combined: boolean;
    /** Franja pegada arriba: una fila de 16 px por barra, sin fechas; en celular, solo el ícono. */
    compact: boolean;
    grid: StripGridLines;
    actions: BarActions;
}

/**
 * Una fila por alerta. Escritorio: nombre, código, duración y fechas exactas a la izquierda; la barra a la derecha.
 * Celular: nombre y duración, la barra a todo el ancho y debajo el código con las fechas.
 */
export function Bar({ bar, combined, compact, grid, actions }: BarProps) {
    const Icon = EVENT_ICON[bar.event];
    const highlighted = combined && bar.current;
    const dimmed = combined && !bar.current;
    const duration = formatDuration(bar.durationMs);
    const range = barRangeLabel(bar);
    const open = () => actions.onOpenAlert(bar.id);
    const rowProps = {
        role: "group",
        "aria-label": barAccessibleLabel(bar, barName(bar)),
        onPointerEnter: dimmed ? () => actions.onPrefetchAlert(bar.id) : undefined,
    };
    const track = (
        <Track bar={bar} dimmed={dimmed} cutColorClass={highlighted ? "border-muted" : "border-background"} grid={grid} actions={actions} />
    );
    const nameClass = highlighted ? "font-semibold text-gray-900" : "text-gray-700";

    if (compact) {
        return (
            <div {...rowProps} className={cn(ROW_PAD_CLASS, COMPACT_ROW_GRID_CLASS, "h-4", highlighted && CURRENT_ROW_CLASS)}>
                <span className={cn("flex min-w-0 items-center gap-1.5 text-xs whitespace-nowrap", nameClass)}>
                    <Icon className="size-3.5 shrink-0" aria-hidden />
                    {/* Con varias alertas el ícono dice el tipo y el código cuál es. */}
                    <span className="truncate max-sm:sr-only">{bar.code ?? EVENT_LABEL[bar.event]}</span>
                    <span className="font-data-mono font-semibold text-gray-900 max-sm:sr-only">({duration})</span>
                </span>
                {track}
            </div>
        );
    }

    const name = (
        <>
            <span className={cn("flex items-center gap-1.5 text-sm", nameClass)}>
                <Icon className="size-3.5 shrink-0" aria-hidden />
                <span className="truncate">{EVENT_LABEL[bar.event]}</span>
                <span className="ml-auto font-data-mono text-xs font-semibold text-gray-900 sm:hidden">{duration}</span>
            </span>
            <span className="hidden font-data-mono text-xs text-gray-600 sm:block">
                {bar.code && `${bar.code} · `}
                <span className="font-semibold text-gray-900">{duration}</span>
            </span>
            <span className="hidden font-data-mono text-xs text-gray-600 sm:block">{range}</span>
        </>
    );
    return (
        <div {...rowProps} className={cn(ROW_PAD_CLASS, EXPANDED_ROW_GRID_CLASS, "flex flex-col gap-1 py-1.5", highlighted && CURRENT_ROW_CLASS)}>
            {dimmed ? (
                <button
                    type="button"
                    onClick={open}
                    aria-label={`Abrir alerta ${bar.code ?? ""}`.trim()}
                    className={cn("flex min-w-0 cursor-pointer flex-col gap-0.5 rounded-sm text-left hover:[&>span:first-child]:underline", FOCUS_CLASS)}
                >
                    {name}
                </button>
            ) : (
                <div className="flex min-w-0 flex-col gap-0.5">{name}</div>
            )}
            {track}
            <span className="font-data-mono text-xs text-gray-600 sm:hidden">
                {bar.code && `${bar.code} · `}
                {range}
            </span>
        </div>
    );
}
