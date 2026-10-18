import { History } from "lucide-react";

export function HistoryPage() {
    return (
        <section className="flex flex-col items-center justify-center gap-3 rounded-xl border border-dashed border-gray-300 py-20 text-center">
            <History size={32} className="text-gray-400" aria-hidden />
            <h2 className="text-lg font-semibold text-gray-900">Historial de Alertas</h2>
            <p className="max-w-md text-sm text-gray-500">
                Aquí se listarán las alertas registradas y pendientes de clasificar. Esta sección está en construcción.
            </p>
        </section>
    );
}
