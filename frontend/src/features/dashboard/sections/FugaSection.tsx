import { useMemo } from "react";
import type { Series } from "@/services/dashboard/series.types";
import { useSeries } from "../hooks/useSeries";
import { SeriesChart } from "../charts/SeriesChart";
import { FUGA_SECTION, FUGA_VARIABLES } from "../config/seriesSections";

interface FugaSectionProps {
    deviceId: number;
    startDate: string;
    endDate: string;
}

export function FugaSection({ deviceId, startDate, endDate }: FugaSectionProps) {
    const { data, isLoading, isError, error } = useSeries(deviceId, startDate, endDate, FUGA_VARIABLES);

    // Mismo patrón que ya usas en MetricsAccordion: solo se recalcula si `data` cambia,
    // así cada SeriesChart recibe arrays estables y React.memo sirve.
    const charts = useMemo(() => {
        if (!data) return [];
        const byKey = new Map(data.series.map((s) => [s.key, s]));
        return FUGA_SECTION.charts.map((chart) => ({
            ...chart,
            series: chart.seriesKeys.map((k) => byKey.get(k)).filter((s): s is Series => !!s),
        }));
    }, [data]);

    if (isLoading) return <p className="text-xs text-gray-400 py-4 text-center">Cargando…</p>;
    if (isError) return <p className="text-xs text-red-500 py-4 text-center">{error?.message}</p>;

    return (
        <div className="flex flex-col gap-4">
            {charts.map((c) => (
                <SeriesChart key={c.title} title={c.title} series={c.series} thresholds={c.thresholds} />
            ))}
        </div>
    );
}