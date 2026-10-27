import { useQueryClient } from "@tanstack/react-query";
import type { AlertSearchFilters } from "@/services/alertManagement";
import { alertKeys } from "../../hooks/alertKeys";
import { useHistorySearch } from "../../hooks/useHistorySearch";
import { historySearchKey } from "../../lib/historySearchParams";
import { AlertFiltersForm } from "./AlertFiltersForm";

/** Conecta el formulario de filtros con la búsqueda aplicada (URL). */
export function AlertHistoryFilters() {
    const { filters, applyFilters, clearFilters } = useHistorySearch();
    const queryClient = useQueryClient();

    // "Buscar" siempre trae datos frescos, aunque se repitan los mismos filtros.
    function refreshHistory() {
        void queryClient.invalidateQueries({ queryKey: alertKeys.histories() });
    }

    function handleApply(next: AlertSearchFilters) {
        applyFilters(next);
        refreshHistory();
    }

    function handleClear() {
        clearFilters();
        refreshHistory();
    }

    // Si la búsqueda aplicada cambia por fuera del formulario (Atrás/Adelante del navegador, un enlace),
    // el formulario se reinicia con ella en lugar de quedar mostrando filtros que ya no aplican.
    return (
        <AlertFiltersForm
            key={historySearchKey({ filters, page: 1 })}
            applied={filters}
            onApply={handleApply}
            onClear={handleClear}
        />
    );
}
