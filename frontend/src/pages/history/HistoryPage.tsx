import { useId } from "react";
import { AlertHistorySection } from "@/features/alerts";

export function HistoryPage() {
    const titleId = useId();

    return (
        <section aria-labelledby={titleId} className="flex flex-col gap-4">
            <header>
                <h2 id={titleId} className="text-lg font-semibold text-gray-900">
                    Historial de alertas
                </h2>
                <p className="text-sm text-gray-500">Consulta, gestión y exportación de todas las alertas generadas.</p>
            </header>

            <AlertHistorySection />
        </section>
    );
}
