//Cómo se dibuja una gráfica
import { memo, useMemo, useState } from "react";
import {
    ResponsiveContainer, LineChart, Line, XAxis, YAxis, Tooltip,
    CartesianGrid, Legend, Brush, ReferenceArea, ReferenceLine,
} from "recharts";
import type { Series } from "@/services/dashboard/series.types";
import type { ThresholdBand } from "../config/seriesSections";

const LINE_COLORS = ["#3b82f6", "#f59e0b", "#10b981", "#ef4444", "#8b5cf6"];
const BUCKET_MS = 10_000;
const NO_THRESHOLDS: ThresholdBand[] = []; // referencia estable para que memo funcione

const cleanUnit = (unit: string) => (unit === "-" ? "" : unit);

const axisDateFmt = new Intl.DateTimeFormat("es-CO", { day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit" });
const tooltipDateFmt = new Intl.DateTimeFormat("es-CO", { day: "2-digit", month: "2-digit", year: "numeric", hour: "2-digit", minute: "2-digit", second: "2-digit" });
const yFmtBig = new Intl.NumberFormat("es-CO", { maximumFractionDigits: 0 });
const yFmtMid = new Intl.NumberFormat("es-CO", { maximumFractionDigits: 2 });
const yFmtSmall = new Intl.NumberFormat("es-CO", { maximumFractionDigits: 3 });
const valueFmt = new Intl.NumberFormat("es-CO", { maximumFractionDigits: 4 });

const formatAxisDate = (ts: number) => axisDateFmt.format(ts);
const formatTooltipDate = (ts: number) => tooltipDateFmt.format(ts);
function formatYTick(value: number) {
    const abs = Math.abs(value);
    return (abs >= 100 ? yFmtBig : abs >= 1 ? yFmtMid : yFmtSmall).format(value);
}

// Sin recorte por percentil: en detección de fuga el pico es justo lo que se quiere ver.
function getYDomain(values: number[], thresholds: ThresholdBand[]): [number, number] {
    if (values.length === 0) return [0, 1];
    let min = Math.min(...values);
    let max = Math.max(...values);
    for (const t of thresholds) {
        min = Math.min(min, t.from);
        if (t.to !== undefined) max = Math.max(max, t.to);
    }
    const range = max - min;
    const padding = range === 0 ? Math.abs(max) * 0.1 || 1 : range * 0.1;
    const lower = min >= 0 ? Math.max(0, min - padding) : min - padding;
    return [lower, max + padding];
}

interface SeriesChartProps {
    title: string;
    series: Series[];
    thresholds?: ThresholdBand[];
    colors?: string[];
}

export const SeriesChart = memo(function SeriesChart({
    title, series, thresholds = NO_THRESHOLDS, colors = LINE_COLORS,
}: SeriesChartProps) {

    const [hiddenKeys, setHiddenKeys] = useState<Set<string>>(new Set());

    const { data, yDomain } = useMemo(() => {
        const bucketMap = new Map<number, Record<string, number>>();
        const allValues: number[] = [];
        for (const s of series) {
            for (const point of s.data) {
                const ts = new Date(point.timestamp).getTime();
                const bucket = Math.round(ts / BUCKET_MS) * BUCKET_MS;
                const row = bucketMap.get(bucket) ?? { timestamp: bucket };
                row[s.key] = point.value;
                bucketMap.set(bucket, row);
                allValues.push(point.value);
            }
        }
        const rows = [...bucketMap.values()].sort((a, b) => a.timestamp - b.timestamp);
        return { data: rows, yDomain: getYDomain(allValues, thresholds) };
    }, [series, thresholds]);

    const unit = cleanUnit(series[0]?.unit ?? "");

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
            <p className="mb-2 flex items-center gap-2 text-sm font-medium text-gray-900">
                <span className="h-2 w-2 rounded-full" style={{ backgroundColor: colors[0] }} />
                {title}
                {unit ? ` (${unit})` : ""}
            </p>
            {data.length === 0 ? (
                <p className="text-xs text-gray-400 py-8 text-center">Sin datos en este rango</p>
            ) : (
                <ResponsiveContainer width="100%" height={series.length > 1 ? 300 : 240} debounce={150}>
                    <LineChart data={data} margin={{ left: 4, right: 12 }}>
                        <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" />
                        {thresholds.map((band) => (
                            <ReferenceArea
                                key={band.label}
                                y1={band.from}
                                y2={band.to ?? yDomain[1]}
                                fill={band.color}
                                fillOpacity={0.08}
                                ifOverflow="hidden"
                                label={{ value: band.label, position: "insideTopRight", fontSize: 10, fill: band.color }}
                            />
                        ))}
                        {thresholds.slice(1).map((band) => (
                            <ReferenceLine key={`line-${band.label}`} y={band.from} stroke={band.color} strokeDasharray="4 4" />
                        ))}
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
                                const u = cleanUnit(series.find((s) => s.name === name)?.unit ?? "");
                                const v = valueFmt.format(Number(value));
                                return [`${v}${u ? ` ${u}` : ""}`, String(name)];
                            }}
                        />
                        {series.length > 1 && (
                            <Legend
                                wrapperStyle={{ fontSize: 11, cursor: "pointer" }}
                                onClick={(entry) => toggleKey(entry.dataKey as string)}
                                formatter={(value, entry) => {
                                    const hidden = hiddenKeys.has(entry?.dataKey as string);
                                    return (
                                        <span style={{ textDecoration: hidden ? "line-through" : "none", opacity: hidden ? 0.5 : 1 }}>
                                            {value}
                                        </span>
                                    );
                                }}
                            />
                        )}
                        {series.map((s, i) => (
                            <Line
                                key={s.key}
                                type="monotone"
                                dataKey={s.key}
                                name={s.name}
                                stroke={colors[i % colors.length]}
                                strokeWidth={1.5}
                                dot={data.length <= 30 ? { r: 2.5 } : false}
                                isAnimationActive={false}
                                connectNulls
                                hide={hiddenKeys.has(s.key)}
                            />
                        ))}
                        {data.length > 20 && (
                            <Brush dataKey="timestamp" height={24} stroke={colors[0]} tickFormatter={formatAxisDate} travellerWidth={8} />
                        )}
                    </LineChart>
                </ResponsiveContainer>
            )}
        </div>
    );
});