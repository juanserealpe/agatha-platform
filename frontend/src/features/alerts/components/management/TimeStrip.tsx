import { useMemo, useState } from "react";
import { TooltipProvider } from "@/components/ui/tooltip";
import { useElementSize } from "@/shared/hooks/useElementSize";
import { useIsStuck } from "@/shared/hooks/useIsStuck";
import { useNow } from "@/shared/hooks/useNow";
import { cn } from "@/lib/utils";
import type { PhaseRef } from "../../hooks/useTimelineNavigation";
import {
    buildStripCursor,
    buildTimeStrip,
    compactStripHeightPx,
    type HoverRange,
    type StripCursor,
    type StripIndicatorInput,
    type TimeStrip as TimeStripModel,
} from "../../lib/timeStrip";
import { TimeStripAxis } from "./TimeStripAxis";
import { Bar } from "./TimeStripBar";
import { CursorBand } from "./TimeStripCursor";
import { TRACK_INSET_CLASS } from "./timeStripStyles";

interface StripPanelProps {
    strip: TimeStripModel;
    cursor: StripCursor | null;
    selected: PhaseRef | null;
    onSelect: (target: PhaseRef) => void;
    /** Pegada arriba al hacer scroll: sin eje de fechas, una fila por barra con su nombre (§5.3). */
    compact: boolean;
}

function StripPanel({ strip, cursor, selected, onSelect, compact }: StripPanelProps) {
    const { bars } = strip;
    // La banda cubre el eje (si se ve) y todas las barras de la columna de pistas (en móvil expandida, también
    // los nombres, que van sobre cada barra). Va con posición absoluta sobre esas celdas: como celda normal de la
    // grilla desplazaba a las demás, la franja crecía y el contenido saltaba bajo el cursor (bucle infinito).
    const bandRows = compact
        ? ({ "--rows": bars.length, "--rows-sm": bars.length } as React.CSSProperties)
        : ({ "--rows": 1 + 2 * bars.length, "--rows-sm": 1 + bars.length } as React.CSSProperties);
    return (
        <section
            aria-label="Franja de tiempo general"
            className={cn("rounded-lg border border-border bg-background px-3", compact ? "flex h-full flex-col justify-center py-2.5" : "py-2.5")}
        >
            {/* Expandida en pantallas angostas: el nombre va sobre su barra para que la barra conserve el ancho. */}
            <div
                className={cn(
                    "relative grid items-center gap-x-3",
                    compact
                        ? "grid-cols-[auto_minmax(0,1fr)] gap-y-1"
                        : "grid-cols-1 gap-y-1.5 sm:grid-cols-[auto_minmax(0,1fr)] sm:gap-y-2.5"
                )}
            >
                {!compact && (
                    <>
                        <span className="hidden sm:block" aria-hidden />
                        <div className={TRACK_INSET_CLASS}>
                            <TimeStripAxis ticks={strip.ticks} />
                        </div>
                    </>
                )}
                {bars.map((bar, index) => (
                    <Bar key={bar.id} bar={bar} isFirst={index === 0} compact={compact} selected={selected} onSelect={onSelect} />
                ))}
                {cursor && (
                    <CursorBand
                        cursor={cursor}
                        className={cn(
                            "absolute inset-0 row-start-1 [grid-row-end:span_var(--rows)] sm:col-start-2 sm:[grid-row-end:span_var(--rows-sm)]",
                            compact ? "col-start-2" : "col-start-1"
                        )}
                        style={bandRows}
                    />
                )}
            </div>
        </section>
    );
}

interface TimeStripProps {
    /** Una entrada por alerta; hoy siempre una (la vista combinada llega en la etapa 4). */
    indicators: readonly StripIndicatorInput[];
    selected: PhaseRef | null;
    /** Clic en un tramo de color: navegar a su fase (§5.3). Nunca cambia datos. */
    onSelectPhase: (target: PhaseRef) => void;
    /** Rango bajo el cursor en la línea de tiempo, para la banda del cursor de tiempo. */
    hoverRange: HoverRange | null;
    /** Clases del contenedor fijo, p. ej. para cubrir el relleno lateral del modal al quedar pegado. */
    className?: string;
}

/**
 * Franja de tiempo general (spec multiindicador §5): eje común y una barra por alerta, sin marcas de eventos
 * (D9). Queda fija arriba al hacer scroll y se compacta: sin fechas, las barras siguen navegando.
 */
export function TimeStrip({ indicators, selected, onSelectPhase, hoverRange, className }: TimeStripProps) {
    const now = useNow();
    const strip = useMemo(() => buildTimeStrip(indicators, now), [indicators, now]);
    const cursor = useMemo(() => (strip && hoverRange ? buildStripCursor(strip, hoverRange) : null), [strip, hoverRange]);
    const [sentinelRef, stuck] = useIsStuck<HTMLSpanElement>();
    const [measureRef, { height }] = useElementSize<HTMLDivElement>();
    // Alto de la franja completa, recordado mientras está compacta (patrón de React para derivar de una medida).
    const [fullHeight, setFullHeight] = useState(0);
    if (!stuck && height !== fullHeight) setFullHeight(height);
    if (!strip) return null;

    const compactHeight = compactStripHeightPx(strip.bars.length);

    return (
        <TooltipProvider>
            <div
                ref={measureRef}
                className={cn("sticky top-0 z-10 bg-background", stuck && "py-2", className)}
                // Compacta: ocupa poco arriba, pero el margen conserva el alto completo en el flujo para que el
                // contenido no salte al compactarse.
                style={stuck ? { height: compactHeight, marginBottom: Math.max(0, fullHeight - compactHeight) } : undefined}
            >
                <span ref={sentinelRef} aria-hidden className="pointer-events-none absolute inset-x-0 -top-0.5 h-px" />
                <StripPanel strip={strip} cursor={cursor} selected={selected} onSelect={onSelectPhase} compact={stuck} />
            </div>
        </TooltipProvider>
    );
}
