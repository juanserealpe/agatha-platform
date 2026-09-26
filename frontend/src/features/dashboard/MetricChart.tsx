/* eslint-disable @typescript-eslint/no-unused-vars */
// features/dashboard/MetricChart.tsx
import { memo, useMemo, useState } from "react";
import {
    ResponsiveContainer,
    LineChart,
    Line,
    XAxis,
    YAxis,
    Tooltip,
    CartesianGrid,
    Legend,
    Brush,
} from "recharts";
import { METRIC_UNIT_OVERRIDES } from "./metricUnits";
import type { MetricMeasurements } from "@/services/dashboard/measurement.types";

const LINE_COLORS = ["#3b82f6", "#f59e0b", "#10b981", "#ef4444", "#8b5cf6"];
const BURST_BUCKET_MS = 10_000; // agrupa lecturas de la misma ráfaga (~1-2s de diferencia entre métricas)

function formatAxisDate(ts: number) {
    return new Date(ts).toLocaleString("es-CO", { day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit" });
}
function formatTooltipDate(ts: number) {
    return new Date(ts).toLocaleString("es-CO", { day: "2-digit", month: "2-digit", year: "numeric", hour: "2-digit", minute: "2-digit", second: "2-digit" });
}

function getYDomain(values: number[]): [number, number] {
    if (values.length === 0) return [0, 1];
    const sorted = [...values].sort((a, b) => a - b);
    const percentile = (p: number) => sorted[Math.min(sorted.length - 1, Math.floor((p / 100) * sorted.length))];
    const min = percentile(1);
    const max = percentile(99);
    const range = max - min;
    const padding = range === 0 ? Math.abs(max) * 0.1 || 1 : range * 0.15;
    return [min - padding, max + padding];
}

function formatYTick(value: number): string {
    return value.toLocaleString("es-CO", {
        maximumFractionDigits: Math.abs(value) >= 100 ? 0 : 2,
    });
}

interface MetricChartProps {
    title: string;
    metrics: MetricMeasurements[];
}

export const MetricChart = memo(function MetricChart({ title, metrics }: MetricChartProps) {
    const { data, yDomain, unit } = useMemo(() => {
        const bucketMap = new Map<number, Record<string, number>>();
        const allValues: number[] = [];
        const unit = METRIC_UNIT_OVERRIDES[metrics[0]?.metricId] ?? metrics[0]?.unit ?? "";

        for (const metric of metrics) {
            for (const point of metric.data) {
                const ts = new Date(point.timestamp).getTime();
                const bucket = Math.round(ts / BURST_BUCKET_MS) * BURST_BUCKET_MS;
                const row = bucketMap.get(bucket) ?? { timestamp: bucket };
                row[`m${metric.metricId}`] = point.value;
                bucketMap.set(bucket, row);
                allValues.push(point.value);
            }
        }

        const rows = Array.from(bucketMap.values()).sort((a, b) => a.timestamp - b.timestamp);
        return {
            data: rows,
            yDomain: getYDomain(allValues),
            unit,
        };
    }, [metrics]);

    const [hiddenKeys, setHiddenKeys] = useState<Set<string>>(new Set());

    function toggleKey(key: string) {
        setHiddenKeys((prev) => {
            const next = new Set(prev);
            if (next.has(key)) next.delete(key);
            else next.add(key);
            return next;
        });
    }

    return (
        <div className="border border-gray-200 rounded-lg p-3">
            <p className="text-sm font-medium text-gray-900 mb-2">
                {title}
                {unit ? ` (${unit})` : ""}
            </p>
            <ResponsiveContainer width="100%" height={metrics.length > 1 ? 300 : 240} debounce={150}>
                <LineChart data={data} margin={{ left: 4, right: 12 }}>
                    <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" />
                    <XAxis
                        dataKey="timestamp"
                        type="number"
                        domain={["dataMin", "dataMax"]}
                        tickFormatter={formatAxisDate}
                        tick={{ fontSize: 11 }}
                        minTickGap={30}
                    />
                    <YAxis domain={yDomain} tick={{ fontSize: 11 }} width={50} tickFormatter={formatYTick} />
                    <Tooltip
                        labelFormatter={(ts) => formatTooltipDate(ts as number)}
                        formatter={(value, name) => {
                            const metric = metrics.find((m) => m.metricName === name);
                            const metricUnit = metric ? METRIC_UNIT_OVERRIDES[metric.metricId] ?? metric.unit : "";
                            const displayValue = typeof value === "number" ? value : Number(value);
                            return [`${displayValue}${metricUnit ? ` ${metricUnit}` : ""}`, String(name)];
                        }}
                    />
                    {metrics.length > 1 && (
                        <Legend
                            wrapperStyle={{ fontSize: 11, cursor: "pointer" }}
                            onClick={(entry) => toggleKey(entry.dataKey as string)}
                            formatter={(value, entry) => {
                                const dataKey = entry?.dataKey as string | undefined;
                                const metric = metrics.find((m) => `m${m.metricId}` === dataKey);
                                const metricUnit = metric ? METRIC_UNIT_OVERRIDES[metric.metricId] ?? metric.unit : "";
                                const isHidden = dataKey ? hiddenKeys.has(dataKey) : false;
                                return (
                                    <span style={{ textDecoration: isHidden ? "line-through" : "none", opacity: isHidden ? 0.5 : 1 }}>
                                        {value}
                                        {metricUnit ? ` (${metricUnit})` : ""}
                                    </span>
                                );
                            }}
                        />
                    )}
                    {metrics.map((metric, i) => (
                        <Line
                            key={metric.metricId}
                            type="monotone"
                            dataKey={`m${metric.metricId}`}
                            name={metric.metricName}
                            stroke={LINE_COLORS[i % LINE_COLORS.length]}
                            strokeWidth={1.5}
                            dot={false}
                            isAnimationActive={false}
                            connectNulls
                            hide={hiddenKeys.has(`m${metric.metricId}`)}
                        />
                    ))}
                    {data.length > 20 && (
                        <Brush dataKey="timestamp" height={24} stroke="#3b82f6" tickFormatter={formatAxisDate} travellerWidth={8} />
                    )}
                </LineChart>
            </ResponsiveContainer>
        </div>
    );
});