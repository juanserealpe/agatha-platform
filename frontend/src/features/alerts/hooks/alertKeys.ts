import type { AlertId, AlertSearchFilters } from "@/services/alertManagement";

// Fábrica de query keys: invalidar alertKeys.all refresca todas las consultas de alertas.
export const alertKeys = {
    all: ["alerts"] as const,
    /** Panel "Sin clasificar" (GET /api/alerts/pending). */
    pending: () => [...alertKeys.all, "pending"] as const,
    /** Puntos del mapa con alertas activas (GET /api/alerts/active). */
    active: () => [...alertKeys.all, "active"] as const,
    /** Todas las páginas de todas las búsquedas del historial. */
    histories: () => [...alertKeys.all, "history"] as const,
    /** Una página de la búsqueda con los filtros APLICADOS. */
    history: (filters: AlertSearchFilters, page: number) => [...alertKeys.histories(), filters, page] as const,
    /** Detalle de gestión (GET /api/alerts/{id}). */
    management: (id: AlertId) => [...alertKeys.all, "management", id] as const,
    /** Alertas del mismo punto que se cruzan con la alerta abierta (vista combinada). */
    group: (id: AlertId) => [...alertKeys.all, "group", id] as const,
};
