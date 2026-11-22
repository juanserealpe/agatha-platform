import { cn } from "@/lib/utils";
import { Skeleton, SkeletonBlock } from "@/shared/components/Skeleton";
import { CHART_HEIGHT_PX } from "../config/chartLayout";
import { QUICK_RANGE_OPTIONS } from "../lib/dateRanges";

const LOADING_TEXT = "Cargando gráficas…";

/** Lo que el esqueleto necesita saber de cada gráfica de la sección: si ocupa las dos columnas. */
interface ChartSlot {
    wide?: boolean;
}

// Sin la configuración de la sección (el esqueleto se pinta antes de descargarla), se insinúan dos gráficas.
const DEFAULT_SLOTS: readonly ChartSlot[] = [{}, {}];

/** Una gráfica: cabecera (título, descripción y último valor) y el área de trazado con su alto real. */
function ChartBones() {
    return (
        <div className="flex flex-col gap-2 rounded-lg border border-slate-200 bg-white p-3">
            <div className="flex items-start justify-between gap-3">
                <div className="flex min-w-0 flex-col gap-1.5">
                    <SkeletonBlock className="h-4 w-36" />
                    <SkeletonBlock className="h-3 w-48" />
                </div>
                <SkeletonBlock className="h-6 w-16 shrink-0" />
            </div>
            <SkeletonBlock className="w-full rounded-md" style={{ height: CHART_HEIGHT_PX }} />
        </div>
    );
}

/** Leyenda y grilla de gráficas, con la misma disposición que SeriesSection (una o dos columnas según el ancho). */
function ChartGridBones({ slots }: { slots: readonly ChartSlot[] }) {
    return (
        <div className="@container flex flex-col gap-2">
            <div className="flex flex-wrap gap-x-4 gap-y-1">
                <SkeletonBlock className="h-3 w-24" />
                <SkeletonBlock className="h-3 w-56" />
            </div>
            <div className="grid grid-cols-1 gap-4 @2xl:grid-cols-2">
                {slots.map((slot, index) => (
                    <div key={index} className={cn("min-w-0", slot.wide && "@2xl:col-span-2")}>
                        <ChartBones />
                    </div>
                ))}
            </div>
        </div>
    );
}

/** Gráficas de una sección mientras llegan sus datos (los controles de rango ya están). */
export function ChartGridSkeleton({ slots = DEFAULT_SLOTS }: { slots?: readonly ChartSlot[] }) {
    return (
        <Skeleton label={LOADING_TEXT}>
            <ChartGridBones slots={slots} />
        </Skeleton>
    );
}

/** Sección completa mientras se descarga el código de las gráficas: controles de rango y gráficas. */
export function SeriesSectionSkeleton() {
    return (
        <Skeleton label={LOADING_TEXT} className="flex flex-col gap-3">
            <div className="flex flex-wrap items-center gap-2">
                {QUICK_RANGE_OPTIONS.map((option) => (
                    <SkeletonBlock key={option.value} className="h-8 w-11 rounded-lg" />
                ))}
                <SkeletonBlock className="h-8 w-32 rounded-lg" />
            </div>
            <ChartGridBones slots={DEFAULT_SLOTS} />
        </Skeleton>
    );
}
