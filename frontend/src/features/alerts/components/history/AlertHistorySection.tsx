import { useCallback } from "react";
import { AlertCircle, SearchX } from "lucide-react";
import type { AlertId } from "@/services/alertManagement";
import { userErrorMessage } from "@/services/http";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/shared/components/EmptyState";
import { Pagination } from "@/shared/components/Pagination";
import { Skeleton, SkeletonBlock } from "@/shared/components/Skeleton";
import { useDevices } from "@/features/sensors";
import { useAlertDetailParam } from "../../hooks/useAlertDetailParam";
import { deviceOptionLabel } from "../../lib/deviceLabel";
import { summarizeFilters } from "../../lib/filterSummary";
import { ROW_ACTION_MODE } from "../../lib/managementMode";
import type { RowAction } from "../../lib/rowActions";
import { historySearchKey } from "../../lib/historySearchParams";
import { ExportAlertsButton } from "./ExportAlertsButton";
import { useAlertHistory } from "../../hooks/useAlertHistory";
import { usePrefetchAlertView } from "../../hooks/useAlertGroup";
import { useHistorySearch } from "../../hooks/useHistorySearch";
import { AlertHistoryTable, AlertHistoryTableSkeleton } from "./AlertHistoryTable";

const LOAD_ERROR = "No fue posible consultar las alertas. Intente nuevamente.";
// Filas del esqueleto: el tamaño de página por defecto del backend, así la tabla no cambia de alto al llegar.
const SKELETON_ROWS = 10;

/** Primera carga: contador, botón de exportar, tabla y paginación en su lugar, con bloques en vez de datos. */
function HistorySkeleton() {
    return (
        <Skeleton label="Cargando alertas…" className="flex flex-col gap-3">
            <div className="flex flex-wrap items-start justify-between gap-3">
                <SkeletonBlock className="mt-1.5 h-4 w-40" />
                <SkeletonBlock className="h-8 w-36 rounded-lg" />
            </div>
            <div className="overflow-hidden rounded-xl border border-border bg-white shadow-xs">
                <AlertHistoryTableSkeleton rows={SKELETON_ROWS} />
            </div>
            <SkeletonBlock className="h-3 w-32" />
        </Skeleton>
    );
}

/** Resultado de la búsqueda aplicada: contador, tabla, paginación y sus estados de carga, vacío y error. */
export function AlertHistorySection() {
    const { filters, page, goToPage } = useHistorySearch();
    const { data, error, isPending, isError, isFetching, isPlaceholderData, refetch, totalPages } = useAlertHistory(filters, page);
    const { open: openAlert } = useAlertDetailParam();
    // Al acercarse a una fila: su detalle y su grupo, para que el modal abra ya con sus pestañas.
    const prefetchAlertView = usePrefetchAlertView();
    const { data: devices = [] } = useDevices();
    const summary = summarizeFilters(filters, (id) => deviceOptionLabel(id, devices));

    // El id es el de la ALERTA; cada acción abre el modal en su modo (clasificar, etiquetar o lectura).
    const handleAction = useCallback(
        (id: AlertId, action: RowAction) => openAlert(String(id), ROW_ACTION_MODE[action]),
        [openAlert]
    );

    if (isPending) return <HistorySkeleton />;

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
            <div className="flex flex-wrap items-start justify-between gap-3">
                <p className="pt-1.5 text-sm text-gray-600" aria-live="polite">
                    Número de alertas: <span className="font-data-mono font-bold text-gray-900">{total}</span>
                    {summary.length > 0 && <span className="text-gray-500"> · {summary.join(" · ")}</span>}
                </p>
                {/* Con otra búsqueda se reinicia, para no arrastrar el error de la exportación anterior. */}
                <ExportAlertsButton key={historySearchKey({ filters, page: 1 })} filters={filters} total={total} />
            </div>

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
                            onIntent={prefetchAlertView}
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
