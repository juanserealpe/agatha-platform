import { cn } from "@/lib/utils";
import { NOW_LABEL } from "../../lib/timeStrip";
import type { AxisTick } from "../../lib/timeAxis";

// Cerca de los bordes la etiqueta se pega hacia adentro en vez de centrarse, para no salirse de la pista.
const EDGE_ALIGN_PCT = 6;

function alignClass(offsetPct: number): string {
    if (offsetPct < EDGE_ALIGN_PCT) return "translate-x-0";
    if (offsetPct > 100 - EDGE_ALIGN_PCT) return "-translate-x-full";
    return "-translate-x-1/2";
}

interface TimeStripAxisProps {
    ticks: readonly AxisTick[];
    nowOffsetPct: number | null;
}

/**
 * Etiquetas del eje (ajuste de la etapa 4): marcas regulares, no los eventos de las alertas, así que nunca se
 * amontonan. La medianoche muestra el día ("jue 19"); el resto, la hora. Sus líneas van en `TimeStripGrid`.
 */
export function TimeStripAxis({ ticks, nowOffsetPct }: TimeStripAxisProps) {
    return (
        <div className="relative h-4 font-data-mono text-xs whitespace-nowrap text-gray-500" aria-hidden>
            {ticks.map((tick) => (
                <span
                    key={tick.atMs}
                    className={cn("absolute top-0", alignClass(tick.offsetPct), tick.isMajor && "font-semibold text-gray-700")}
                    style={{ left: `${tick.offsetPct}%` }}
                >
                    {tick.label}
                </span>
            ))}
            {nowOffsetPct !== null && (
                <span
                    className={cn("absolute top-0 font-semibold text-gray-900", alignClass(nowOffsetPct))}
                    style={{ left: `${nowOffsetPct}%` }}
                >
                    {NOW_LABEL}
                </span>
            )}
        </div>
    );
}

interface TimeStripGridProps {
    ticks: readonly AxisTick[];
    nowOffsetPct: number | null;
}

/** Líneas verticales tenues de cada marca y la punteada de "Ahora", sobre el eje y todas las barras. */
export function TimeStripGrid({ ticks, nowOffsetPct }: TimeStripGridProps) {
    return (
        <>
            {ticks.map((tick) => (
                <span
                    key={tick.atMs}
                    className={cn("absolute inset-y-0 w-px", tick.isMajor ? "bg-gray-900/15" : "bg-gray-900/8")}
                    style={{ left: `${tick.offsetPct}%` }}
                />
            ))}
            {nowOffsetPct !== null && (
                <span className="absolute inset-y-0 border-l border-dashed border-gray-500" style={{ left: `${nowOffsetPct}%` }} />
            )}
        </>
    );
}
