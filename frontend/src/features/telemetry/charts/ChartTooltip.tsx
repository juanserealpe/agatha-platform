import type { TooltipContentProps } from "recharts";
import type { LeakThresholds, Series } from "@/services/dashboard/series.types";
import { formatAbsolute, formatShort } from "@/shared/lib/formatDate";
import { LEAK_LEVEL_STYLES } from "../config/leakThresholds";
import { leakLevelOf } from "../lib/leakLevel";
import { formatWithUnit } from "../lib/formatValue";

const EVALUATION_WINDOW_MS = 24 * 60 * 60 * 1000;

interface ChartTooltipProps extends Pick<TooltipContentProps, "active" | "payload" | "label"> {
    series: Series[];
    colors: string[];
    leakThresholds?: LeakThresholds;
}

export function ChartTooltip({ active, payload, label, series, colors, leakThresholds }: ChartTooltipProps) {
    if (!active || !payload) return null;
    const entries = payload.filter((p) => typeof p.value === "number");
    if (entries.length === 0) return null;

    const ts = Number(label);
    const isEvaluation = series.some((s) => s.source === "evaluation");

    return (
        <div className="min-w-44 rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs shadow-md">
            <p className="font-medium text-slate-900">{formatAbsolute(ts)}</p>
            {isEvaluation && (
                // El punto resume la ventana que termina en su timestamp, no ese instante.
                <p className="text-slate-500">Ventana: {formatShort(ts - EVALUATION_WINDOW_MS)} → {formatShort(ts)}</p>
            )}
            <ul className="mt-1.5 flex flex-col gap-1">
                {entries.map((entry) => {
                    const index = series.findIndex((s) => s.key === entry.dataKey);
                    const s = series[index];
                    const value = entry.value as number;
                    return (
                        <li key={String(entry.dataKey)} className="flex items-center justify-between gap-3">
                            <span className="flex items-center gap-1.5 text-slate-600">
                                <span className="h-2 w-2 rounded-full" style={{ backgroundColor: colors[index] }} />
                                {s?.name ?? entry.name}
                            </span>
                            <span className="font-data-mono font-semibold text-slate-900">{formatWithUnit(value, s?.unit ?? "")}</span>
                        </li>
                    );
                })}
            </ul>
            {leakThresholds && (
                <TooltipLevel value={entries[0].value as number} thresholds={leakThresholds} />
            )}
        </div>
    );
}

function TooltipLevel({ value, thresholds }: { value: number; thresholds: LeakThresholds }) {
    const { label, badgeClass } = LEAK_LEVEL_STYLES[leakLevelOf(value, thresholds)];
    return (
        <p className="mt-2 flex items-center gap-1.5 text-slate-500">
            Zona: <span className={`rounded px-1.5 py-0.5 font-medium ${badgeClass}`}>{label}</span>
        </p>
    );
}
