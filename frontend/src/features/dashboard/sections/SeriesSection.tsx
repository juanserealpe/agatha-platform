import { useMemo } from "react";
import type { Series } from "@/services/dashboard/series.types";
import { useSeries } from "../hooks/useSeries";
import { useDateRangeState } from "../hooks/useDateRangeState";
import { DateRangeControls } from "../components/DateRangeControls";
import { SeriesChart } from "../charts/SeriesChart";
import { SERIES_SECTIONS } from "../config/seriesSections";
import type { SectionKey } from "../config/sectionStyles";

interface SeriesSectionProps {
    sectionKey: SectionKey;
    deviceId: number;
}

export function SeriesSection({ sectionKey, deviceId }: SeriesSectionProps) {
    const config = SERIES_SECTIONS[sectionKey];
    const { quickRange, startDate, endDate, selectQuickRange, selectCustomRange } = useDateRangeState();
    const { data, isLoading, isError, error, isFetching } = useSeries(deviceId, startDate, endDate, config.variables);

    const charts = useMemo(() => {
        if (!data) return [];
        const byKey = new Map(data.series.map((s) => [s.key, s]));
        return config.charts.map((chart) => ({
            ...chart,
            series: chart.seriesKeys.map((k) => byKey.get(k)).filter((s): s is Series => !!s),
        }));
    }, [data, config]);

    return (
        <div className="flex flex-col gap-3">
            <DateRangeControls
                activeQuickRange={quickRange}
                onQuickRangeSelect={selectQuickRange}
                onCustomRangeApply={selectCustomRange}
            />
            {isLoading ? (
                <p className="py-4 text-center text-xs text-gray-400">Cargando…</p>
            ) : isError ? (
                <p className="py-4 text-center text-xs text-red-500">{error?.message}</p>
            ) : (
                <div className={`@container transition-opacity ${isFetching ? "opacity-70" : ""}`}>
                    <div className="grid grid-cols-1 gap-4 @2xl:grid-cols-2">
                        {charts.map((c) => (
                            <div key={c.title} className={`min-w-0 ${c.wide ? "@2xl:col-span-2" : ""}`}>
                                <SeriesChart title={c.title} series={c.series} thresholds={c.thresholds} colors={c.colors} />
                            </div>
                        ))}
                    </div>
                </div>
            )}
        </div>
    );
}