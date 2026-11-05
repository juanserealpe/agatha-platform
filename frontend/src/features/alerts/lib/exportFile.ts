import { MANAGEMENT_LIMITS } from "@/services/alertManagement";
import { formatColombiaFileStamp } from "@/shared/lib/formatDate";

// Mismo texto que responde el backend con 400: se muestra antes de pedir un archivo que no se generará.
export const EXPORT_TOO_LARGE_MESSAGE = "La búsqueda tiene más de 10.000 alertas. Ajuste los filtros para exportar.";

/**
 * Nombre del archivo: el de `Content-Disposition` si llega, o el mismo formato del backend
 * (`alertas_AAAAMMDD_HHMM.xlsx`, hora de Colombia). En producción el CORS del backend aún no expone esa
 * cabecera, así que el respaldo es lo habitual.
 */
export function exportFilename(serverName: string | null, now: Date = new Date()): string {
    return serverName ?? `alertas_${formatColombiaFileStamp(now)}.xlsx`;
}

export function exceedsExportLimit(total: number): boolean {
    return total > MANAGEMENT_LIMITS.exportMaxAlerts;
}
