import { useId } from "react";
import { History } from "lucide-react";
import { AlertHistoryFilters, AlertHistorySection } from "@/features/alerts";
import { BrandBubbles } from "@/shared/components/BrandBubbles";

export function HistoryPage() {
    const titleId = useId();

    return (
        <section aria-labelledby={titleId} className="flex flex-col gap-4">
            {/* Banner con los colores de TGI: un velo suave y los círculos de marca asomándose por la derecha. */}
            <header className="relative flex items-center gap-3 overflow-hidden rounded-xl border border-tgi-verde/25 bg-linear-to-r from-tgi-verde/15 via-tgi-lima/5 to-transparent px-4 py-3">
                <span className="flex size-10 shrink-0 items-center justify-center rounded-full bg-white/70 text-tgi-verde ring-1 ring-tgi-verde/25">
                    <History size={20} aria-hidden />
                </span>
                <div className="relative">
                    <h2 id={titleId} className="text-lg font-semibold text-gray-900">
                        Historial de alertas
                    </h2>
                    <p className="text-sm text-gray-600">Consulta, gestión y exportación de todas las alertas generadas.</p>
                </div>
                <BrandBubbles className="absolute top-1/2 -right-6 hidden h-28 -translate-y-1/2 sm:block" />
            </header>

            <AlertHistoryFilters />
            <AlertHistorySection />
        </section>
    );
}
