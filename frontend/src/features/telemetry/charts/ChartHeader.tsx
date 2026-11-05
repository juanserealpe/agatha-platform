import { useId, useState } from "react";
import { Info } from "lucide-react";
import type { LeakThresholds } from "@/services/dashboard/series.types";
import { formatAbsolute, formatTimestamp, toIsoString } from "@/shared/lib/formatDate";
import { LEAK_LEVEL_STYLES } from "../config/leakThresholds";
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
}

export function ChartHeader({ title, unit, color, description, howToRead, summary, leakThresholds }: ChartHeaderProps) {
    const [showHelp, setShowHelp] = useState(false);
    const helpId = useId();
    const u = cleanUnit(unit);

    return (
        <header className="mb-2 flex flex-col gap-1">
            <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                    <h3 className="flex items-center gap-2 text-sm font-semibold text-slate-900">
                        <span className="h-2 w-2 shrink-0 rounded-full" style={{ backgroundColor: color }} />
                        {title}
                        {u && <span className="font-normal text-slate-500">({u})</span>}
                        {howToRead.length > 0 && (
                            <button
                                type="button"
                                aria-label={showHelp ? "Ocultar cómo leer esta gráfica" : "Cómo leer esta gráfica"}
                                aria-expanded={showHelp}
                                aria-controls={helpId}
                                onClick={() => setShowHelp((v) => !v)}
                                className={`rounded-full p-0.5 hover:bg-slate-100 hover:text-slate-700 focus-visible:outline-2 focus-visible:outline-blue-500 ${showHelp ? "text-blue-600" : "text-slate-400"}`}
                            >
                                <Info size={14} />
                            </button>
                        )}
                    </h3>
                    <p className="text-xs text-slate-500">{description}</p>
                </div>
                {summary && <LatestValue summary={summary} unit={unit} leakThresholds={leakThresholds} />}
            </div>
            {showHelp && (
                <ul id={helpId} className="mt-1 list-disc space-y-1 rounded-md bg-slate-50 py-2 pr-3 pl-7 text-xs text-slate-600">
                    {howToRead.map((line) => <li key={line}>{line}</li>)}
                </ul>
            )}
        </header>
    );
}

function LatestValue({ summary, unit, leakThresholds }: { summary: SeriesSummary; unit: string; leakThresholds?: LeakThresholds }) {
    const { last, min, max } = summary;
    const level = leakThresholds ? LEAK_LEVEL_STYLES[leakLevelOf(last.value, leakThresholds)] : null;

    return (
        <div className="flex shrink-0 flex-col items-end text-right">
            <span className="flex items-center gap-2">
                {level && <span className={`rounded px-1.5 py-0.5 text-xs font-medium ${level.badgeClass}`}>{level.label}</span>}
                <span className="font-data-mono text-lg leading-none font-semibold text-slate-900">{formatWithUnit(last.value, unit)}</span>
            </span>
            <time dateTime={toIsoString(last.timestamp)} title={formatAbsolute(last.timestamp)} className="text-xs text-slate-500">
                Último · {formatTimestamp(last.timestamp)}
            </time>
            <span className="font-data-mono text-xs text-slate-400">
                mín {formatWithUnit(min, unit)} · máx {formatWithUnit(max, unit)}
            </span>
        </div>
    );
}
