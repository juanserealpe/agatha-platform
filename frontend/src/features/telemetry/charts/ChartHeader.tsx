import { useId, useState } from "react";
import { Info } from "lucide-react";
import type { LeakThresholds } from "@/services/dashboard/series.types";
import { cn } from "@/lib/utils";
import { formatAbsolute, formatTimestamp, toIsoString } from "@/shared/lib/formatDate";
import { CHART_INK } from "../config/chartPalette";
import { LEAK_LEVEL_STYLES } from "../config/leakThresholds";
import type { ReferenceMark, ThresholdBand } from "../config/seriesSections";
import { leakLevelOf } from "../lib/leakLevel";
import { cleanUnit, formatWithUnit } from "../lib/formatValue";
import type { SeriesSummary } from "../lib/chartData";

interface ChartHeaderProps {
    title: string;
    unit: string;
    color: string;
    description: string;
    howToRead: string[];
    summary: SeriesSummary | null;
    leakThresholds?: LeakThresholds;
    thresholds: ThresholdBand[];
    references: ReferenceMark[];
}

/** Título y descripción arriba; debajo, el último valor en grande y la clave de umbrales y referencias. */
export function ChartHeader({ title, unit, color, description, howToRead, summary, leakThresholds, thresholds, references }: ChartHeaderProps) {
    const [showHelp, setShowHelp] = useState(false);
    const helpId = useId();
    const u = cleanUnit(unit);

    return (
        <header className="mb-3 flex flex-col gap-2.5">
            <div className="min-w-0">
                <h3 className="flex items-center gap-1.5 text-sm font-semibold text-slate-900">
                    <span className="size-2 shrink-0 rounded-full" style={{ backgroundColor: color }} aria-hidden />
                    <span className="min-w-0">
                        {title}
                        {u && <span className="font-normal text-slate-500"> ({u})</span>}
                    </span>
                    {howToRead.length > 0 && (
                        <button
                            type="button"
                            aria-label={showHelp ? "Ocultar cómo leer esta gráfica" : "Cómo leer esta gráfica"}
                            aria-expanded={showHelp}
                            aria-controls={helpId}
                            onClick={() => setShowHelp((v) => !v)}
                            className={cn(
                                "shrink-0 rounded-full p-0.5 hover:bg-slate-200/70 hover:text-slate-700 focus-visible:outline-2 focus-visible:outline-blue-500",
                                showHelp ? "text-blue-600" : "text-slate-400"
                            )}
                        >
                            <Info size={14} aria-hidden />
                        </button>
                    )}
                </h3>
                <div className="mt-0.5 text-xs text-slate-500">{description}</div>
            </div>
            {showHelp && (
                <ul id={helpId} className="list-disc space-y-1 rounded-md bg-white py-2 pr-3 pl-7 text-xs text-slate-600 ring-1 ring-slate-200/80">
                    {howToRead.map((line) => <li key={line}>{line}</li>)}
                </ul>
            )}
            {summary && <LatestValue summary={summary} unit={unit} leakThresholds={leakThresholds} />}
            {(thresholds.length > 0 || references.length > 0) && <ChartKey thresholds={thresholds} references={references} />}
        </header>
    );
}

function LatestValue({ summary, unit, leakThresholds }: { summary: SeriesSummary; unit: string; leakThresholds?: LeakThresholds }) {
    const { last, min, max } = summary;
    const level = leakThresholds ? LEAK_LEVEL_STYLES[leakLevelOf(last.value, leakThresholds)] : null;

    return (
        <div className="flex flex-col gap-1">
            {/* div y no p: el contenido del acordeón de shadcn da margen inferior a todo <p> que no sea el último. */}
            <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
                <span className="font-data-mono text-2xl leading-none font-semibold tracking-tight text-slate-900">
                    {formatWithUnit(last.value, unit)}
                </span>
                {level && <span className={cn("rounded-md px-1.5 py-0.5 text-xs font-medium", level.badgeClass)}>{level.label}</span>}
            </div>
            {/* Sin "·" entre los dos grupos: en un teléfono el segundo baja de línea y el separador quedaría colgando. */}
            <p className="flex flex-wrap gap-x-3 text-xs text-slate-500">
                <time dateTime={toIsoString(last.timestamp)} title={formatAbsolute(last.timestamp)}>
                    Último · {formatTimestamp(last.timestamp)}
                </time>
                <span className="font-data-mono">
                    mín {formatWithUnit(min, unit)} · máx {formatWithUnit(max, unit)}
                </span>
            </p>
        </div>
    );
}

/** Línea punteada de muestra, igual a la que se dibuja en la gráfica. */
function DashSwatch({ color, dash }: { color: string; dash: string }) {
    return (
        <svg width="16" height="6" viewBox="0 0 16 6" aria-hidden className="shrink-0">
            <line x1="0" y1="3" x2="16" y2="3" stroke={color} strokeWidth="2" strokeDasharray={dash} />
        </svg>
    );
}

// Las etiquetas de umbrales y referencias van aquí y no dentro del trazado, donde tapaban la línea.
function ChartKey({ thresholds, references }: { thresholds: ThresholdBand[]; references: ReferenceMark[] }) {
    return (
        <ul className="flex flex-wrap gap-x-3 gap-y-1 text-xs text-slate-600" aria-label="Umbrales y referencias">
            {thresholds.map((band) => (
                <li key={band.label} className="flex items-center gap-1.5">
                    <DashSwatch color={band.color} dash="4 2" />
                    {band.label}
                </li>
            ))}
            {references.map((ref) => (
                <li key={ref.label} className="flex items-center gap-1.5">
                    <DashSwatch color={CHART_INK.reference} dash="2 2" />
                    {ref.label}
                </li>
            ))}
        </ul>
    );
}
