import { useMemo, useState } from "react";
import { TooltipProvider } from "@/components/ui/tooltip";
import { useElementSize } from "@/shared/hooks/useElementSize";
import { useIsStuck } from "@/shared/hooks/useIsStuck";
import { useMediaQuery } from "@/shared/hooks/useMediaQuery";
import { useNow } from "@/shared/hooks/useNow";
import { cn } from "@/lib/utils";
import type { PhaseRef } from "../../hooks/useTimelineNavigation";
import { computeTimeTicks } from "../../lib/timeAxis";
import {
    buildStripCursor,
    buildTimeStrip,
    compactStripHeightPx,
    type HoverRange,
    type StripCursor,
    type StripIndicatorInput,
    type TimeStrip as TimeStripModel,
} from "../../lib/timeStrip";
import { TimeStripAxis, TimeStripGrid } from "./TimeStripAxis";
import { Bar, type BarActions } from "./TimeStripBar";
import { CursorBand } from "./TimeStripCursor";
import {
    COMPACT_OVERLAY_X_CLASS,
    EXPANDED_OVERLAY_X_CLASS,
    EXPANDED_ROW_GRID_CLASS,
    ROW_PAD_CLASS,
    TRACK_INSET_CLASS,
} from "./timeStripStyles";

// Tailwind `sm`: desde aquí el eje muestra "jue 19"; más angosto, solo "19" y marcas más juntas.
const WIDE_SCREEN_QUERY = "(min-width: 640px)";

/** Capa sobre la columna de las barras (mismo ancho que sus pistas). No recibe clics. */
function Overlay({ className, children }: { className: string; children: React.ReactNode }) {
    return (
        <div aria-hidden className={cn("pointer-events-none absolute bottom-0 z-20", className)}>
            <div className={cn("size-full", TRACK_INSET_CLASS)}>
                <div className="relative size-full">{children}</div>
            </div>
        </div>
    );
}

interface StripPanelProps {
    strip: TimeStripModel;
    cursor: StripCursor | null;
    actions: BarActions;
    /** Pegada arriba al hacer scroll: sin eje de fechas, una fila por barra con su nombre (§5.3). */
    compact: boolean;
}

function StripPanel({ strip, cursor, actions, compact }: StripPanelProps) {
    const { bars } = strip;
    const combined = bars.length > 1;
    const overlayX = compact ? COMPACT_OVERLAY_X_CLASS : EXPANDED_OVERLAY_X_CLASS;
    const wide = useMediaQuery(WIDE_SCREEN_QUERY);
    const [axisRef, { width: trackWidth }] = useElementSize<HTMLDivElement>();
    const ticks = useMemo(
        () =>
            computeTimeTicks({
                startMs: strip.startMs,
                endMs: strip.endMs,
                widthPx: trackWidth,
                narrow: !wide,
                nowMs: strip.nowMs,
            }),
        [strip, trackWidth, wide]
    );
    // En la compacta no hay fechas: las líneas de las marcas no dirían nada; "Ahora" sí se mantiene.
    const grid = useMemo(() => ({ ticks: compact ? [] : ticks, nowOffsetPct: strip.nowOffsetPct }), [compact, ticks, strip.nowOffsetPct]);

    return (
        <section
            aria-label="Franja de tiempo general"
            className={cn("rounded-lg border border-border bg-background px-1", compact ? "flex h-full flex-col justify-center py-2.5" : "py-2")}
        >
            <div className={cn("relative flex flex-col", compact ? "gap-y-1" : "gap-y-0.5")}>
                {!compact && (
                    <div className={cn(ROW_PAD_CLASS, EXPANDED_ROW_GRID_CLASS)}>
                        <span className="hidden sm:block" aria-hidden />
                        <div ref={axisRef} className={TRACK_INSET_CLASS}>
                            <TimeStripAxis ticks={ticks} nowOffsetPct={strip.nowOffsetPct} />
                        </div>
                    </div>
                )}
                {bars.map((bar) => (
                    <Bar key={bar.id} bar={bar} combined={combined} compact={compact} grid={grid} actions={actions} />
                ))}
                {/* Escritorio: líneas del eje y "Ahora" de lado a lado, bajo las etiquetas y solo sobre la columna de
                    las barras (los nombres quedan limpios). En celular cada barra dibuja las suyas (TimeStripBar),
                    porque ahí nombres y fechas van sobre y bajo la barra. */}
                <Overlay className={cn("hidden sm:block", compact ? "top-0" : "top-4.5", overlayX)}>
                    <TimeStripGrid {...grid} />
                </Overlay>
                {cursor && (
                    <Overlay className={cn("top-0 z-30", overlayX)}>
                        <CursorBand cursor={cursor} />
                    </Overlay>
                )}
            </div>
        </section>
    );
}

interface TimeStripProps {
    /** Una entrada por alerta del grupo (vista combinada); una sola si ninguna otra se cruza con la abierta. */
    indicators: readonly StripIndicatorInput[];
    selected: PhaseRef | null;
    /** Clic en un tramo de la alerta abierta: navegar a su fase (§5.3). Nunca cambia datos. */
    onSelectPhase: (target: PhaseRef) => void;
    /** Clic en otra alerta del grupo: abrirla en el modal. */
    onOpenAlert: (alertId: number) => void;
    onPrefetchAlert: (alertId: number) => void;
    /** Rango bajo el cursor en la línea de tiempo, para la banda del cursor de tiempo. */
    hoverRange: HoverRange | null;
    /** Clases del contenedor fijo, p. ej. para cubrir el relleno lateral del modal al quedar pegado. */
    className?: string;
}

/**
 * Franja de tiempo general (spec multiindicador §5): eje de marcas regulares y una fila por alerta con sus fechas
 * exactas (ajuste de la etapa 4). Queda fija arriba al hacer scroll y se compacta: sin fechas, las barras siguen
 * navegando.
 */
export function TimeStrip({ indicators, selected, onSelectPhase, onOpenAlert, onPrefetchAlert, hoverRange, className }: TimeStripProps) {
    const now = useNow();
    const strip = useMemo(() => buildTimeStrip(indicators, now), [indicators, now]);
    const cursor = useMemo(() => (strip && hoverRange ? buildStripCursor(strip, hoverRange) : null), [strip, hoverRange]);
    const actions = useMemo<BarActions>(
        () => ({ selected, onSelect: onSelectPhase, onOpenAlert, onPrefetchAlert }),
        [selected, onSelectPhase, onOpenAlert, onPrefetchAlert]
    );
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
                <StripPanel strip={strip} cursor={cursor} actions={actions} compact={stuck} />
            </div>
        </TooltipProvider>
    );
}
