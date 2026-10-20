//Cómo se dibuja una gráfica
import { memo, useId, useMemo, type ComponentProps } from "react";
import {
    ResponsiveContainer, ComposedChart, Area, Line, XAxis, YAxis, Tooltip,
    CartesianGrid, Legend, ReferenceArea, ReferenceLine,
} from "recharts";
import type { LeakThresholds, Series } from "@/services/dashboard/series.types";
import { formatClock, formatDayMonth } from "@/shared/lib/formatDate";
import type { ReferenceMark, ThresholdBand, YScale } from "../config/seriesSections";
import { CHART_INK, MULTI_SERIES_COLORS, SOURCE_COLORS } from "../config/chartPalette";
import { buildChartRows, computeTimeTicks, computeYDomain, computeYTicks, isDayTick, nearestIndex, summarize } from "../lib/chartData";
import { makeTickFormatter } from "../lib/formatValue";
import { ChartHeader } from "./ChartHeader";
import { ChartTooltip } from "./ChartTooltip";

const CHART_HEIGHT = 220;
// Con pocos puntos (p. ej. 1 día del indicador horario) cada punto se marca: son valores discretos.
const MAX_POINTS_WITH_DOTS = 48;
// Con muchas lecturas (una semana de B1 son ~650) un trazo fino deja ver la forma en vez de un bloque.
const DENSE_POINTS = 300;
const AXIS_TICK = { fontSize: 12, fill: CHART_INK.axis };
const NO_BANDS: ThresholdBand[] = [];
const NO_REFERENCES: ReferenceMark[] = [];

// El cursor se sincroniza por instante (no por índice): las gráficas tienen cadencias distintas.
// Recharts no exporta el tipo SyncMethod desde su índice.
type SyncMethod = Extract<ComponentProps<typeof ComposedChart>["syncMethod"], (...args: never[]) => number>;
const syncByTime: SyncMethod = (ticks, data) =>
    data.activeLabel === undefined ? -1 : nearestIndex(ticks.map((t) => Number(t.value)), Number(data.activeLabel));

interface SeriesChartProps {
    title: string;
    description: string;
    howToRead: string[];
    series: Series[];
    /** Ventana pedida: todas las gráficas de la sección comparten el mismo eje X. */
    range: { start: number; end: number };
    syncId: string;
    thresholds?: ThresholdBand[];
    leakThresholds?: LeakThresholds;
    references?: ReferenceMark[];
    yScale?: YScale;
}

export const SeriesChart = memo(function SeriesChart({
    title, description, howToRead, series, range, syncId,
    thresholds = NO_BANDS, leakThresholds, references = NO_REFERENCES, yScale,
}: SeriesChartProps) {
    const gradientId = useId();
    const colors = series.length === 1 ? [SOURCE_COLORS[series[0].source]] : MULTI_SERIES_COLORS;

    const { rows, summary, yDomain, yTicks, formatYTick, ticks } = useMemo(() => {
        const summaries = series.map((s) => summarize(s.data)).filter((s) => s !== null);
        const min = Math.min(...summaries.map((s) => s.min));
        const max = Math.max(...summaries.map((s) => s.max));
        const yDomain = computeYDomain({ min, max, scale: yScale, thresholds, references });
        const yTicks = computeYTicks(yDomain);
        return {
            rows: buildChartRows(series),
            summary: summaries[0] ?? null,
            yDomain,
            yTicks,
            formatYTick: makeTickFormatter(yTicks.length > 1 ? yTicks[1] - yTicks[0] : 1),
            ticks: computeTimeTicks(range.start, range.end),
        };
    }, [series, yScale, thresholds, references, range]);

    const unit = series[0]?.unit ?? "";
    const hasData = summary !== null;
    const formatXTick = (ts: number) => (isDayTick(ts, ticks.stepMs) ? formatDayMonth(ts) : formatClock(ts));

    return (
        <section className="rounded-lg border border-slate-200 bg-white p-3" aria-label={title}>
            <ChartHeader
                title={title}
                unit={unit}
                color={colors[0]}
                description={description}
                howToRead={howToRead}
                summary={series.length === 1 ? summary : null}
                leakThresholds={leakThresholds}
            />
            {!hasData ? (
                <p className="py-10 text-center text-xs text-slate-400">
                    {series[0]?.source === "evaluation" ? "Aún no se ha calculado para este rango." : "Sin lecturas en este rango."}
                </p>
            ) : (
                <ResponsiveContainer width="100%" height={CHART_HEIGHT} debounce={150}>
                    <ComposedChart data={rows} margin={{ top: 8, left: 0, right: 8, bottom: 0 }} syncId={syncId} syncMethod={syncByTime}>
                        <defs>
                            <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
                                <stop offset="0%" stopColor={colors[0]} stopOpacity={0.22} />
                                <stop offset="100%" stopColor={colors[0]} stopOpacity={0.02} />
                            </linearGradient>
                        </defs>
                        <CartesianGrid vertical={false} stroke={CHART_INK.grid} />
                        {thresholds.map((band) => (
                            <ReferenceArea
                                key={`area-${band.label}`}
                                y1={band.from}
                                y2={band.to ?? yDomain[1]}
                                fill={band.color}
                                fillOpacity={0.1}
                                ifOverflow="hidden"
                            />
                        ))}
                        {thresholds.map((band) => (
                            <ReferenceLine
                                key={`line-${band.label}`}
                                y={band.from}
                                stroke={band.color}
                                strokeWidth={1.5}
                                strokeDasharray="6 3"
                                label={{ value: band.label, position: "insideTopRight", fontSize: 12, fill: CHART_INK.reference }}
                            />
                        ))}
                        {references.map((ref) => (
                            <ReferenceLine
                                key={ref.label}
                                y={ref.value}
                                stroke={CHART_INK.reference}
                                strokeDasharray="3 3"
                                label={{ value: ref.label, position: "insideTopRight", fontSize: 12, fill: CHART_INK.reference }}
                            />
                        ))}
                        <XAxis
                            dataKey="timestamp"
                            type="number"
                            scale="time"
                            domain={[range.start, range.end]}
                            ticks={ticks.ticks}
                            tickFormatter={formatXTick}
                            tick={AXIS_TICK}
                            tickLine={false}
                            axisLine={{ stroke: CHART_INK.grid }}
                            allowDataOverflow
                        />
                        <YAxis
                            domain={yDomain}
                            ticks={yTicks}
                            tick={AXIS_TICK}
                            tickFormatter={formatYTick}
                            width={44}
                            tickLine={false}
                            axisLine={false}
                            allowDataOverflow
                        />
                        <Tooltip
                            cursor={{ stroke: CHART_INK.reference, strokeDasharray: "3 3" }}
                            isAnimationActive={false}
                            content={(props) => (
                                <ChartTooltip {...props} series={series} colors={colors} leakThresholds={leakThresholds} />
                            )}
                        />
                        {series.length > 1 && <Legend wrapperStyle={{ fontSize: 12 }} />}
                        {series.map((s, i) => {
                            const common = {
                                type: "monotone" as const,
                                dataKey: s.key,
                                name: s.name,
                                stroke: colors[i % colors.length],
                                strokeWidth: rows.length > DENSE_POINTS ? 1.25 : 2,
                                dot: rows.length <= MAX_POINTS_WITH_DOTS ? { r: 2.5, strokeWidth: 0, fill: colors[i % colors.length] } : false,
                                activeDot: { r: 4, strokeWidth: 2, stroke: "#fff" },
                                isAnimationActive: false,
                            };
                            return series.length === 1
                                ? <Area key={s.key} {...common} fill={`url(#${gradientId})`} baseValue={yDomain[0]} />
                                : <Line key={s.key} {...common} />;
                        })}
                    </ComposedChart>
                </ResponsiveContainer>
            )}
        </section>
    );
});
