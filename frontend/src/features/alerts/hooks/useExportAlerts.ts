import { useMutation } from "@tanstack/react-query";
import { alertManagementService, type AlertSearchFilters } from "@/services/alertManagement";
import { downloadBlob } from "@/shared/lib/downloadBlob";
import { exportFilename } from "../lib/exportFile";

/**
 * Exporta TODA la búsqueda aplicada (sin paginar) a .xlsx y la descarga. Si el backend responde error
 * (p. ej. más de 10.000 alertas) la mutación falla con su `message` y no se descarga nada.
 */
export function useExportAlerts() {
    return useMutation({
        mutationFn: (filters: AlertSearchFilters) => alertManagementService.exportXlsx(filters),
        onSuccess: ({ blob, filename }) => downloadBlob(blob, exportFilename(filename)),
    });
}
