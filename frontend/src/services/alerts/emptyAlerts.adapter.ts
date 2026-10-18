import type { AlertsService } from "./alerts.service";

/**
 * Implementación sin datos mientras el backend no expone alertas.
 * No hace peticiones ni inventa registros: la UI muestra sus estados vacíos.
 */
export const emptyAlertsService: AlertsService = {
    getRecent: async () => [],
    getPending: async () => [],
    getById: async () => null,
};
