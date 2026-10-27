import { useId } from "react";
import { History } from "lucide-react";
import { AlertHistoryFilters, AlertHistorySection } from "@/features/alerts";

export function HistoryPage() {
    const titleId = useId();

    return (
        <section aria-labelledby={titleId} className="flex flex-col gap-4">
            {/* Banner con un velo muy suave de los colores de TGI: identidad sin competir con la severidad. */}
            <header className="flex items-center gap-3 rounded-xl border border-tgi-verde/15 bg-linear-to-r from-tgi-verde/10 via-tgi-lima/5 to-transparent px-4 py-3">
                <span className="flex size-10 shrink-0 items-center justify-center rounded-full bg-tgi-verde/10 text-tgi-verde">
                    <History size={20} aria-hidden />
                </span>
                <div>
                    <h2 id={titleId} className="text-lg font-semibold text-gray-900">
                        Historial de alertas
                    </h2>
                    <p className="text-sm text-gray-600">Consulta, gestión y exportación de todas las alertas generadas.</p>
                </div>
            </header>

            <AlertHistoryFilters />
            <AlertHistorySection />
        </section>
    );
}
