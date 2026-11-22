import { Skeleton, SkeletonBlock } from "@/shared/components/Skeleton";

// Medidas de react-day-picker (style.css): celdas de 44 px, 7 días por semana y hasta 6 semanas por mes.
const DAYS_PER_WEEK = 7;
const WEEKS_SHOWN = 6;

/** Calendario mientras se descarga su código: título del mes, días de la semana y la grilla del mes. */
export function CalendarSkeleton() {
    return (
        <Skeleton label="Cargando calendario…" className="flex flex-col gap-2 p-3">
            <div className="flex h-11 items-center justify-between">
                <SkeletonBlock className="h-4 w-32" />
                <SkeletonBlock className="h-6 w-16" />
            </div>
            <div className="grid grid-cols-7 gap-0.5">
                {Array.from({ length: DAYS_PER_WEEK * (WEEKS_SHOWN + 1) }, (_, index) => (
                    <div key={index} className="flex size-11 items-center justify-center">
                        <SkeletonBlock className={index < DAYS_PER_WEEK ? "h-3 w-5" : "size-8 rounded-full"} />
                    </div>
                ))}
            </div>
        </Skeleton>
    );
}
