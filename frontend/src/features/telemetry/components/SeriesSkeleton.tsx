import { cn } from "@/lib/utils";
import { Skeleton, SkeletonBlock } from "@/shared/components/Skeleton";
import { CHART_HEIGHT_PX } from "../config/chartLayout";
import { QUICK_RANGE_OPTIONS } from "../lib/dateRanges";

const LOADING_TEXT = "Cargando gráficas…";
// Ancho aproximado de cada botón del control segmentado de rangos.
const RANGE_SEGMENT_PX = 50;

/** Lo que el esqueleto necesita saber de cada gráfica de la sección: si ocupa las dos columnas. */
interface ChartSlot {
    wide?: boolean;
}

// Sin la configuración de la sección (el esqueleto se pinta antes de descargarla), se insinúan dos gráficas.
const DEFAULT_SLOTS: readonly ChartSlot[] = [{}, {}];

/** Una gráfica: título, descripción, último valor con su línea de datos y el área de trazado con su alto real. */
function ChartBones() {
    return (
        <div className="flex flex-col gap-3 rounded-xl bg-slate-50 p-3 sm:p-4">
            <div className="flex flex-col gap-1.5">
                <SkeletonBlock className="h-4 w-36" />
                <SkeletonBlock className="h-3 w-56 max-w-full" />
            </div>
            <div className="flex flex-col gap-1.5">
                <SkeletonBlock className="h-6 w-20" />
                <SkeletonBlock className="h-3 w-48 max-w-full" />
            </div>
            <SkeletonBlock className="w-full rounded-md" style={{ height: CHART_HEIGHT_PX }} />
        </div>
    );
}

/** Grilla de gráficas, con la misma disposición que SeriesSection (una o dos columnas según el ancho). */
function ChartGridBones({ slots }: { slots: readonly ChartSlot[] }) {
    return (
        <div className="@container">
            <div className="grid grid-cols-1 gap-3 @2xl:grid-cols-2">
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
        <Skeleton label={LOADING_TEXT} className="flex flex-col gap-4">
            <div className="flex flex-wrap items-center gap-2">
                <SkeletonBlock className="h-8 rounded-lg" style={{ width: QUICK_RANGE_OPTIONS.length * RANGE_SEGMENT_PX }} />
                <SkeletonBlock className="h-8 w-40 rounded-lg" />
            </div>
            <ChartGridBones slots={DEFAULT_SLOTS} />
        </Skeleton>
    );
}
