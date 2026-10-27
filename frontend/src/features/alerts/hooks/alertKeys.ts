import type { AlertId, AlertSearchFilters } from "@/services/alertManagement";

// Fábrica de query keys: invalidar alertKeys.all refresca todas las consultas de alertas.
export const alertKeys = {
    all: ["alerts"] as const,
    recent: (limit: number) => [...alertKeys.all, "recent", limit] as const,
    pending: () => [...alertKeys.all, "pending"] as const,
    /** Todas las páginas de todas las búsquedas del historial. */
    histories: () => [...alertKeys.all, "history"] as const,
    /** Una página de la búsqueda con los filtros APLICADOS. */
    history: (filters: AlertSearchFilters, page: number) => [...alertKeys.histories(), filters, page] as const,
    /** Detalle de gestión (GET /api/alerts/{id}). */
    management: (id: AlertId) => [...alertKeys.all, "management", id] as const,
};
