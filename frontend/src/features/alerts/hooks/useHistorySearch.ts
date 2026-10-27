import { useCallback, useMemo } from "react";
import { useSearchParams } from "react-router";
import type { AlertSearchFilters } from "@/services/alertManagement";
import { EMPTY_FILTERS, historySearchKey, parseHistorySearch, withHistorySearch } from "../lib/historySearchParams";

/**
 * Búsqueda APLICADA del historial (filtros de la última búsqueda ejecutada + página), guardada en la URL.
 * La tabla, la paginación y la exportación leen solo de aquí; los filtros en edición viven en el panel.
 */
export function useHistorySearch() {
    const [searchParams, setSearchParams] = useSearchParams();

    // Clave canónica: abrir el modal (?alerta=) o reordenar valores no cambia la referencia de los filtros,
    // así la query y las filas memoizadas no se recalculan en vano.
    const key = historySearchKey(parseHistorySearch(searchParams));
    const { filters, page } = useMemo(() => parseHistorySearch(new URLSearchParams(key)), [key]);

    const goToPage = useCallback(
        (nextPage: number) => {
            setSearchParams((previous) => withHistorySearch(previous, { ...parseHistorySearch(previous), page: nextPage }));
        },
        [setSearchParams]
    );

    /** Ejecuta una búsqueda: siempre vuelve a la página 1. */
    const applyFilters = useCallback(
        (nextFilters: AlertSearchFilters) => {
            setSearchParams((previous) => withHistorySearch(previous, { filters: nextFilters, page: 1 }));
        },
        [setSearchParams]
    );

    const clearFilters = useCallback(() => applyFilters(EMPTY_FILTERS), [applyFilters]);

    return { filters, page, goToPage, applyFilters, clearFilters };
}
