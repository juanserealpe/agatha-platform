import { useMemo } from "react";
import { useElementSize } from "@/shared/hooks/useElementSize";
import { cn } from "@/lib/utils";
import { placeAxisLabels, tickLabelLines, type PlacedTick, type StripTick } from "../../lib/timeStrip";

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

/** Eje de la franja: una fecha por círculo, con su línea guía hasta la barra (§5.1). */
export function TimeStripAxis({ ticks }: { ticks: readonly StripTick[] }) {
    const [ref, { width }] = useElementSize<HTMLDivElement>();
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
