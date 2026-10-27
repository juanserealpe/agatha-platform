import type { AlertSearchFilters } from "@/services/alertManagement";
import { EVENT_LABEL, levelStyle, MANAGEMENT_STATUS_STYLES } from "../config/alertManagementDisplay";
import { isoDayToDisplay } from "./historyFilterForm";

/**
 * Resumen legible de los filtros APLICADOS, p. ej. ["01/10/2026 – 08/10/2026", "Naranja, Rojo", "Fuga"].
 * Vacío si se está viendo el historial completo.
 */
export function summarizeFilters(filters: AlertSearchFilters, deviceName: (id: number) => string): string[] {
    const parts: string[] = [];

    if (filters.device !== null) parts.push(deviceName(filters.device));

    if (filters.from && filters.to) parts.push(`${isoDayToDisplay(filters.from)} – ${isoDayToDisplay(filters.to)}`);
    else if (filters.from) parts.push(`Desde ${isoDayToDisplay(filters.from)}`);
    else if (filters.to) parts.push(`Hasta ${isoDayToDisplay(filters.to)}`);

    if (filters.maxLevels.length > 0) parts.push(filters.maxLevels.map((level) => levelStyle(level).label).join(", "));
    if (filters.events.length > 0) parts.push(filters.events.map((event) => EVENT_LABEL[event]).join(", "));
    if (filters.statuses.length > 0) {
        parts.push(filters.statuses.map((status) => MANAGEMENT_STATUS_STYLES[status].label).join(", "));
    }

    return parts;
}
