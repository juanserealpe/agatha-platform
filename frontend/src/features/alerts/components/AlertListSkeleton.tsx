import { Skeleton, SkeletonBlock } from "@/shared/components/Skeleton";

// Tarjetas que se insinúan mientras carga la lista: las suficientes para llenar el panel sin prometer cuántas hay.
const PLACEHOLDER_ITEMS = 3;

/** Lista del panel de alertas mientras carga: tarjetas con la forma de AlertListItem. */
export function AlertListSkeleton() {
    return (
        <Skeleton label="Cargando alertas…" className="flex flex-col gap-2">
            {Array.from({ length: PLACEHOLDER_ITEMS }, (_, index) => (
                <div key={index} className="flex flex-col gap-2 rounded-lg border border-gray-200 px-3 py-2.5">
                    <div className="flex items-center gap-2">
                        <SkeletonBlock className="h-5 w-16 rounded-full" />
                        <SkeletonBlock className="h-4 w-32" />
                        <SkeletonBlock className="ml-auto h-3 w-14" />
                    </div>
                    <div className="flex items-center gap-2">
                        <SkeletonBlock className="h-3 w-36" />
                        <SkeletonBlock className="ml-auto h-3 w-20" />
                    </div>
                </div>
            ))}
        </Skeleton>
    );
}
