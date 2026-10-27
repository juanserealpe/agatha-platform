import { useCallback } from "react";
import { AlertCircle, Loader2, SearchX } from "lucide-react";
import type { AlertId } from "@/services/alertManagement";
import { userErrorMessage } from "@/services/http";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/shared/components/EmptyState";
import { Pagination } from "@/shared/components/Pagination";
import { useAlertDetailParam } from "../../hooks/useAlertDetailParam";
import { useAlertHistory } from "../../hooks/useAlertHistory";
import { useHistorySearch } from "../../hooks/useHistorySearch";
import { AlertHistoryTable } from "./AlertHistoryTable";

const LOAD_ERROR = "No fue posible consultar las alertas. Intente nuevamente.";

/** Resultado de la búsqueda aplicada: contador, tabla, paginación y sus estados de carga, vacío y error. */
export function AlertHistorySection() {
    const { filters, page, goToPage } = useHistorySearch();
    const { data, error, isPending, isError, isFetching, isPlaceholderData, refetch, totalPages } = useAlertHistory(filters, page);
    const { open: openAlert } = useAlertDetailParam();

    // El modal de gestión llega en un paso posterior; por ahora toda acción abre el detalle de la alerta.
    const handleAction = useCallback((id: AlertId) => openAlert(String(id)), [openAlert]);

    if (isPending) {
        return <EmptyState Icon={Loader2} title="Cargando alertas…" className="py-16 [&>svg]:animate-spin" />;
    }

    if (isError && !data) {
        return (
            <EmptyState
                Icon={AlertCircle}
                title="No se pudieron cargar las alertas"
                description={userErrorMessage(error, LOAD_ERROR)}
                action={
                    <Button variant="outline" onClick={() => void refetch()}>
                        Reintentar
                    </Button>
                }
                className="py-16"
            />
        );
    }

    const { total, items, page_size: pageSize } = data;
    const firstRow = (data.page - 1) * pageSize + 1;
    const lastRow = firstRow + items.length - 1;

    return (
        <div className="flex flex-col gap-3">
            <p className="text-sm text-gray-600" aria-live="polite">
                Número de alertas: <span className="font-data-mono font-bold text-gray-900">{total}</span>
            </p>

            {isError && (
                <p role="alert" className="rounded-lg border border-gray-300 bg-gray-50 px-3 py-2 text-sm text-gray-700">
                    {userErrorMessage(error, LOAD_ERROR)} Se muestran los últimos resultados disponibles.
                </p>
            )}

            {total === 0 ? (
                <EmptyState Icon={SearchX} title="No se encontraron alertas con los filtros seleccionados" className="py-16" />
            ) : items.length === 0 ? (
                // Página fuera de rango (p. ej. un enlace viejo con ?pagina=9 cuando ya hay menos alertas).
                <EmptyState
                    Icon={SearchX}
                    title="Esta página ya no tiene alertas"
                    action={
                        <Button variant="outline" onClick={() => goToPage(1)}>
                            Ir a la primera página
                        </Button>
                    }
                    className="py-16"
                />
            ) : (
                <>
                    <div className="overflow-hidden rounded-xl border border-border bg-white shadow-xs">
                        <AlertHistoryTable
                            items={items}
                            caption={`Historial de alertas, página ${data.page} de ${totalPages}`}
                            isUpdating={isPlaceholderData && isFetching}
                            onAction={handleAction}
                        />
                    </div>

                    <div className="flex flex-wrap items-center justify-between gap-3">
                        <p className="font-data-mono text-xs text-gray-500">
                            Mostrando {firstRow}–{lastRow} de {total}
                        </p>
                        {totalPages > 1 && (
                            <Pagination page={page} totalPages={totalPages} onPageChange={goToPage} disabled={isPlaceholderData} />
                        )}
                    </div>
                </>
            )}
        </div>
    );
}
