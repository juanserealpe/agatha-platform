import { useMemo } from "react";
import type { Series } from "@/services/dashboard/series.types";
import { cn } from "@/lib/utils";
import { useSeries } from "../hooks/useSeries";
import { useTelemetryRange } from "../hooks/useTelemetryRange";
import { DateRangeControls } from "../components/DateRangeControls";
import { ChartGridSkeleton } from "../components/SeriesSkeleton";
import { SeriesChart } from "../charts/SeriesChart";
import { SERIES_SECTIONS, type SeriesSectionConfig } from "../config/seriesSections";
import { DEFAULT_LEAK_THRESHOLDS } from "../config/leakThresholds";
import { SOURCE_COLORS, SOURCE_LABELS } from "../config/chartPalette";
import { leakThresholdBands } from "../lib/leakLevel";
import type { SectionKey } from "../config/sectionStyles";

interface SeriesSectionProps {
    sectionKey: SectionKey;
    deviceId: number;
}

export function SeriesSection({ sectionKey, deviceId }: SeriesSectionProps) {
    const config = SERIES_SECTIONS[sectionKey];
    if (config.unavailableReason) {
        return <p className="py-4 text-center text-xs text-slate-500">{config.unavailableReason}</p>;
    }
    return <AvailableSeriesSection sectionKey={sectionKey} config={config} deviceId={deviceId} />;
}

function AvailableSeriesSection({ sectionKey, config, deviceId }: SeriesSectionProps & { config: SeriesSectionConfig }) {
    const { quickRange, customRange, startDate, endDate, selectQuickRange, selectCustomRange } = useTelemetryRange();
    const { data, isLoading, isError, error, isFetching } = useSeries(deviceId, startDate, endDate, config.variables);

    const range = useMemo(() => ({ start: Date.parse(startDate), end: Date.parse(endDate) }), [startDate, endDate]);

    const charts = useMemo(() => {
        if (!data) return [];
        const byKey = new Map(data.series.map((s) => [s.key, s]));
        return config.charts.map((chart) => {
            const series = chart.seriesKeys.map((k) => byKey.get(k)).filter((s): s is Series => !!s);
            // Los umbrales de la serie (versionados por punto) mandan sobre la plantilla beta.
            const leakThresholds = chart.leakThresholds ? (series[0]?.thresholds ?? DEFAULT_LEAK_THRESHOLDS) : undefined;
            return {
                ...chart,
                series,
                leakThresholds,
                bands: leakThresholds ? leakThresholdBands(leakThresholds) : undefined,
            };
        });
    }, [data, config]);

    const sources = useMemo(() => [...new Set(charts.flatMap((c) => c.series.map((s) => s.source)))], [charts]);
    const usesBetaThresholds = charts.some((c) => c.leakThresholds === DEFAULT_LEAK_THRESHOLDS);

    return (
        <div className="flex flex-col gap-4">
            <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
                <DateRangeControls
                    activeQuickRange={quickRange}
                    customRange={customRange}
                    onQuickRangeSelect={selectQuickRange}
                    onCustomRangeApply={selectCustomRange}
                />
                <ul className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-slate-500" aria-label="Notas de las gráficas">
                    {sources.map((source) => (
                        <li key={source} className="flex items-center gap-1.5">
                            <span className="size-2 rounded-full" style={{ backgroundColor: SOURCE_COLORS[source] }} aria-hidden />
                            {SOURCE_LABELS[source]}
                        </li>
                    ))}
                    <li>Hora local · un corte en la línea es un periodo sin datos</li>
                    {usesBetaThresholds && <li>Umbrales: valores beta del Rule Engine</li>}
                </ul>
            </div>
            {isLoading ? (
                <ChartGridSkeleton slots={config.charts} />
            ) : isError ? (
                <p className="py-4 text-center text-xs text-red-600">{error?.message}</p>
            ) : (
                <div className={cn("@container transition-opacity", isFetching && "opacity-70")}>
                    <div className="grid grid-cols-1 gap-3 @2xl:grid-cols-2">
                        {charts.map((c) => (
                            <div key={c.title} className={cn("min-w-0", c.wide && "@2xl:col-span-2")}>
                                <SeriesChart
                                    title={c.title}
                                    description={c.description}
                                    howToRead={c.howToRead}
                                    series={c.series}
                                    range={range}
                                    syncId={`telemetry-${sectionKey}`}
                                    thresholds={c.bands}
                                    leakThresholds={c.leakThresholds}
                                    references={c.references}
                                    yScale={c.yScale}
                                />
                            </div>
                        ))}
                    </div>
                </div>
            )}
        </div>
    );
}
