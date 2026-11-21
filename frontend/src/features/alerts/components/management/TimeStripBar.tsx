import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import { EVENT_ICON, EVENT_LABEL, levelStyle } from "../../config/alertManagementDisplay";
import type { PhaseRef } from "../../hooks/useTimelineNavigation";
import { formatDuration } from "../../lib/timeline";
import { markerTooltip, pieceAccessibleLabel, pieceTooltip, type StripBar, type StripMarker, type StripPiece } from "../../lib/timeStrip";
import { edgeRounding, FOCUS_CLASS, pieceColorClass, piecePosition, SELECTED_PIECE_CLASS, TRACK_INSET_CLASS } from "./timeStripStyles";

function Hint({ text, children }: { text: string; children: React.ReactElement }) {
    return (
        <Tooltip>
            <TooltipTrigger render={children} />
            <TooltipContent>{text}</TooltipContent>
        </Tooltip>
    );
}

interface PieceProps {
    piece: StripPiece;
    bar: StripBar;
    className: string;
    selected: boolean;
    onSelect: (target: PhaseRef) => void;
}

function Piece({ piece, bar, className, selected, onSelect }: PieceProps) {
    const label = pieceAccessibleLabel(piece, EVENT_LABEL[bar.event]);
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
    return (
        <Hint text={pieceTooltip(piece)}>
            <button
                type="button"
                aria-label={`${label}. Ir a esta fase`}
                aria-pressed={selected}
                onClick={() => onSelect({ alertId: bar.id, seq })}
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

interface BarProps {
    bar: StripBar;
    isFirst: boolean;
    /** Franja pegada arriba: nombre y barra en la misma fila (en móvil, ícono y duración, sin el nombre). */
    compact: boolean;
    selected: PhaseRef | null;
    onSelect: (target: PhaseRef) => void;
}

export function Bar({ bar, isFirst, compact, selected, onSelect }: BarProps) {
    const Icon = EVENT_ICON[bar.event];
    const track = (
        <div className={TRACK_INSET_CLASS}>
            <div className="relative h-2.5">
                {bar.pieces.map((piece, index) => (
                    <Piece
                        key={`${piece.kind}-${piece.startMs}`}
                        piece={piece}
                        bar={bar}
                        className={edgeRounding(index, bar.pieces.length)}
                        selected={selected?.alertId === bar.id && selected.seq === piece.seq}
                        onSelect={onSelect}
                    />
                ))}
                {bar.markers.map((marker) => (
                    <Marker key={`${marker.kind}-${marker.atMs}`} marker={marker} />
                ))}
            </div>
        </div>
    );
    return (
        <>
            {/* En móvil el nombre va sobre su barra; el de la primera, sobre el eje, para que las líneas guía
                del eje lleguen hasta la barra sin cruzar el texto. */}
            <span
                className={cn("flex items-center gap-1.5 text-xs whitespace-nowrap text-gray-600", isFirst && !compact && "max-sm:-order-1")}
            >
                <Icon className="size-3.5 shrink-0" aria-hidden />
                <span className={cn(compact && "max-sm:sr-only")}>{EVENT_LABEL[bar.event]}</span>
                {/* Duración visible sin pasar el cursor: lo primero que se busca al abrir la alerta. */}
                <span className="font-data-mono font-semibold text-gray-900">({formatDuration(bar.durationMs)})</span>
            </span>
            {track}
        </>
    );
}
