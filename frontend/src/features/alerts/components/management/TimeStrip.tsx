import { useMemo } from "react";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
import { useElementWidth } from "@/shared/hooks/useElementWidth";
import { useNow } from "@/shared/hooks/useNow";
import { cn } from "@/lib/utils";
import { EVENT_ICON, EVENT_LABEL, levelStyle } from "../../config/alertManagementDisplay";
import { formatDuration } from "../../lib/timeline";
import {
    buildTimeStrip,
    markerTooltip,
    pieceAccessibleLabel,
    pieceTooltip,
    placeAxisLabels,
    tickLabelLines,
    type PlacedTick,
    type StripBar,
    type StripIndicatorInput,
    type StripMarker,
    type StripPiece,
    type StripTick,
} from "../../lib/timeStrip";

// Tramo sin alerta (§2.1: #E5E7EB): gris neutro, fuera de la escala de severidad.
const IDLE_PIECE_CLASS = "bg-gray-200";
// Margen lateral para que los círculos del inicio y de "Ahora" no se corten; el eje usa el mismo.
const TRACK_INSET_CLASS = "px-1.5";
// Fechas del eje a 10 px: EXCEPCIÓN aprobada por el usuario (20-nov) al mínimo de 12 px de la app
// (frontend/CLAUDE.md), solo aquí. En alertas que suben de nivel en pocas horas los círculos quedan casi
// juntos y a 12 px las fechas se amontonan. Son un apoyo: la fecha completa está en el tooltip de cada
// círculo y en la línea de tiempo. Única clase para no repartir el tamaño arbitrario por el componente.
const AXIS_LABEL_TEXT_CLASS = "text-[10px] leading-3.5";
// Ancho estimado de una etiqueta en la fuente mono de 10 px (todos los caracteres miden igual), con
// holgura, y separación mínima entre etiquetas: amplia para que las corridas no se lean como un bloque.
const AXIS_CHAR_WIDTH_PX = 6.25;
const AXIS_LABEL_PADDING_PX = 4;
const AXIS_LABEL_GAP_PX = 10;

function axisLabelWidth(tick: StripTick): number {
    const longest = Math.max(...tickLabelLines(tick).map((line) => line.length));
    return longest * AXIS_CHAR_WIDTH_PX + AXIS_LABEL_PADDING_PX;
}
const FOCUS_CLASS = "outline-none focus-visible:ring-2 focus-visible:ring-gray-900 focus-visible:ring-offset-1";

function Hint({ text, children }: { text: string; children: React.ReactElement }) {
    return (
        <Tooltip>
            <TooltipTrigger render={children} />
            <TooltipContent>{text}</TooltipContent>
        </Tooltip>
    );
}

function Piece({ piece, indicatorLabel, className }: { piece: StripPiece; indicatorLabel: string; className?: string }) {
    // Los tramos solo informan; la navegación al hacer clic llega en la etapa 3 (y nunca en los grises, CA7).
    return (
        <Hint text={pieceTooltip(piece)}>
            <span
                tabIndex={0}
                role="img"
                aria-label={pieceAccessibleLabel(piece, indicatorLabel)}
                className={cn(
                    "absolute inset-y-0 min-w-0.5 hover:brightness-95",
                    FOCUS_CLASS,
                    piece.level ? levelStyle(piece.level).dotClass : IDLE_PIECE_CLASS,
                    className
                )}
                style={{ left: `${piece.offsetPct}%`, width: `${piece.widthPct}%` }}
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
                    "absolute top-1/2 size-2.5 -translate-x-1/2 -translate-y-1/2 rounded-full",
                    FOCUS_CLASS,
                    // §5.2: relleno del nivel al que se entra con borde del fondo del panel; "Ahora" va hueco.
                    marker.kind === "now" ? cn("border-2 bg-white", style.borderClass) : cn("ring-2 ring-white", style.dotClass)
                )}
                style={{ left: `${marker.offsetPct}%` }}
            />
        </Hint>
    );
}

function Bar({ bar, isFirst }: { bar: StripBar; isFirst: boolean }) {
    const Icon = EVENT_ICON[bar.event];
    const label = EVENT_LABEL[bar.event];
    return (
        <>
            {/* En móvil el nombre va sobre su barra; el de la primera, sobre el eje, para que las líneas guía
                del eje lleguen hasta la barra sin cruzar el texto. */}
            <span className={cn("flex items-center gap-1.5 text-xs whitespace-nowrap text-gray-600", isFirst && "max-sm:-order-1")}>
                <Icon className="size-3.5 shrink-0" aria-hidden />
                {label}
                {/* Duración visible sin pasar el cursor: lo primero que se busca al abrir la alerta. */}
                <span className="font-data-mono font-semibold text-gray-900">({formatDuration(bar.durationMs)})</span>
            </span>
            <div className={TRACK_INSET_CLASS}>
                <div className="relative h-2.5">
                    {bar.pieces.map((piece, index) => (
                        <Piece
                            key={`${piece.kind}-${piece.startMs}`}
                            piece={piece}
                            indicatorLabel={label}
                            className={cn(index === 0 && "rounded-l-full", index === bar.pieces.length - 1 && "rounded-r-full")}
                        />
                    ))}
                    {bar.markers.map((marker) => (
                        <Marker key={`${marker.kind}-${marker.atMs}`} marker={marker} />
                    ))}
                </div>
            </div>
        </>
    );
}

type LabelAlign = "start" | "center" | "end";

const LABEL_ALIGN_CLASS: Record<LabelAlign, string> = {
    start: "items-start",
    center: "items-center",
    end: "items-end",
};

/** El texto se pega al lado donde está su círculo (bordes de la pista o etiqueta corrida para no chocar). */
function labelAlign({ leftPx, widthPx, centerPx }: PlacedTick): LabelAlign {
    if (centerPx < leftPx + widthPx / 2 - 1) return "start";
    if (centerPx > leftPx + widthPx / 2 + 1) return "end";
    return "center";
}

/** La línea guía sale del punto de la etiqueta más cercano a su círculo: recta siempre que se pueda. */
function leaderStartPx({ leftPx, widthPx, centerPx }: PlacedTick): number {
    return Math.min(leftPx + widthPx, Math.max(leftPx, centerPx));
}

function Axis({ ticks }: { ticks: readonly StripTick[] }) {
    const [ref, width] = useElementWidth<HTMLDivElement>();
    const placed = useMemo(() => placeAxisLabels(ticks, width, axisLabelWidth, AXIS_LABEL_GAP_PX), [ticks, width]);
    return (
        <div ref={ref} className="relative h-10" aria-hidden>
            {placed.map((item) => (
                <span
                    key={item.tick.atMs}
                    className={cn(
                        "absolute top-0 flex flex-col font-data-mono whitespace-nowrap text-gray-500",
                        AXIS_LABEL_TEXT_CLASS,
                        LABEL_ALIGN_CLASS[labelAlign(item)],
                        item.tick.isNow && "font-semibold text-gray-900"
                    )}
                    style={{ left: item.leftPx, width: item.widthPx }}
                >
                    {tickLabelLines(item.tick).map((line) => (
                        <span key={line}>{line}</span>
                    ))}
                </span>
            ))}
            {/* Línea guía de cada etiqueta a su círculo: recta si está encima, en diagonal si se corrió para no
                chocar. Baja por el espacio entre filas hasta tocar la barra. */}
            <div className="absolute inset-x-0 top-7.5 -bottom-1.5 sm:-bottom-2.5">
                <svg className="size-full overflow-visible">
                    {placed.map((item) => (
                        <line
                            key={item.tick.atMs}
                            x1={leaderStartPx(item)}
                            y1="0"
                            x2={item.centerPx}
                            y2="100%"
                            className="stroke-gray-300"
                            strokeWidth={1}
                        />
                    ))}
                </svg>
            </div>
        </div>
    );
}

interface TimeStripProps {
    /** Una entrada por alerta; hoy siempre una (la vista combinada llega en la etapa 4). */
    indicators: readonly StripIndicatorInput[];
}

/** Franja de tiempo general (spec multiindicador §5): eje común y una barra por alerta, sin marcas de eventos (D9). */
export function TimeStrip({ indicators }: TimeStripProps) {
    const now = useNow();
    const strip = useMemo(() => buildTimeStrip(indicators, now), [indicators, now]);
    if (!strip) return null;

    return (
        <TooltipProvider>
            <section aria-label="Franja de tiempo general" className="rounded-lg border border-border px-3 py-2.5">
                {/* En pantallas angostas el nombre va sobre su barra para que la barra conserve todo el ancho. */}
                <div className="grid grid-cols-1 items-center gap-x-3 gap-y-1.5 sm:grid-cols-[auto_minmax(0,1fr)] sm:gap-y-2.5">
                    <span className="hidden sm:block" aria-hidden />
                    <div className={TRACK_INSET_CLASS}>
                        <Axis ticks={strip.ticks} />
                    </div>
                    {strip.bars.map((bar, index) => (
                        <Bar key={bar.id} bar={bar} isFirst={index === 0} />
                    ))}
                </div>
            </section>
        </TooltipProvider>
    );
}
