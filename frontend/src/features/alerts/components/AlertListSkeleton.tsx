import { Skeleton, SkeletonBlock } from "@/shared/components/Skeleton";

// Tarjetas que se insinúan mientras carga la lista: las suficientes para llenar el panel sin prometer cuántas hay.
const PLACEHOLDER_ITEMS = 3;

/** Lista del panel de alertas mientras carga: tarjetas con la forma de AlertListItem. */
export function AlertListSkeleton() {
    return (
        <Skeleton label="Cargando alertas…" className="flex flex-col gap-2">
            {Array.from({ length: PLACEHOLDER_ITEMS }, (_, index) => (
                <div key={index} className="flex gap-2.5 rounded-xl border border-gray-200 px-3 py-2">
                    <SkeletonBlock className="mt-0.5 size-7 shrink-0 rounded-lg" />
                    <div className="flex min-w-0 flex-1 flex-col gap-1.5">
                        <div className="flex items-center gap-2">
                            <SkeletonBlock className="h-4 w-32" />
                            <SkeletonBlock className="ml-auto h-3 w-14" />
                        </div>
                        <SkeletonBlock className="h-3 w-36" />
                        <div className="mt-0.5 flex items-center gap-2">
                            <SkeletonBlock className="h-3 w-14" />
                            <SkeletonBlock className="ml-auto h-5 w-20 rounded-full" />
                        </div>
                    </div>
                </div>
            ))}
        </Skeleton>
    );
}
