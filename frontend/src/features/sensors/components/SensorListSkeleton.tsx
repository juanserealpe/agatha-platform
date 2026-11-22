import { Skeleton, SkeletonBlock } from "@/shared/components/Skeleton";

// Filas que se insinúan mientras carga: las suficientes para llenar el panel sin prometer cuántos sensores hay.
const PLACEHOLDER_ITEMS = 4;

/** Lista de sensores mientras carga: filas con la forma de SensorListItem (estado, nombre, conexión y batería). */
export function SensorListSkeleton() {
    return (
        <Skeleton label="Cargando sensores…" className="flex flex-col gap-2">
            {Array.from({ length: PLACEHOLDER_ITEMS }, (_, index) => (
                <div key={index} className="flex items-center gap-3 rounded-lg border border-gray-200 p-3">
                    <SkeletonBlock className="size-2.5 shrink-0 rounded-full" />
                    <div className="flex min-w-0 flex-1 flex-col gap-1.5">
                        <SkeletonBlock className="h-4 w-32" />
                        <SkeletonBlock className="h-3 w-40" />
                    </div>
                    <SkeletonBlock className="h-4 w-12 shrink-0" />
                </div>
            ))}
        </Skeleton>
    );
}
