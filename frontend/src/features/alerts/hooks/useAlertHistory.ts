import { useEffect } from "react";
import { keepPreviousData, queryOptions, useQuery, useQueryClient } from "@tanstack/react-query";
import { alertManagementService, type AlertSearchFilters } from "@/services/alertManagement";
import { pageCount } from "../lib/historyCache";
import { alertKeys } from "./alertKeys";

// El historial cambia con cada hora procesada, pero no es la vista de tiempo real: medio minuto evita
// repetir la consulta al ir y volver entre páginas sin mostrar datos viejos por mucho tiempo.
const HISTORY_STALE_MS = 30_000;

function historyQuery(filters: AlertSearchFilters, page: number) {
    return queryOptions({
        queryKey: alertKeys.history(filters, page),
        queryFn: ({ signal }) => alertManagementService.search(filters, page, signal),
        staleTime: HISTORY_STALE_MS,
    });
}

/**
 * Página del historial para los filtros APLICADOS (los de la última búsqueda, no los que se están editando).
 * Mientras llega una página nueva se mantiene la anterior para que la tabla no parpadee, y la siguiente
 * se precarga para que avanzar sea inmediato.
 */
export function useAlertHistory(filters: AlertSearchFilters, page: number) {
    const queryClient = useQueryClient();
    const query = useQuery({ ...historyQuery(filters, page), placeholderData: keepPreviousData });

    const totalPages = query.data ? pageCount(query.data.total, query.data.page_size) : 0;
    const hasNextPage = !query.isPlaceholderData && page < totalPages;

    useEffect(() => {
        if (hasNextPage) void queryClient.prefetchQuery(historyQuery(filters, page + 1));
    }, [filters, hasNextPage, page, queryClient]);

    return { ...query, totalPages };
}
