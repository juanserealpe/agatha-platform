import { useQuery } from "@tanstack/react-query";
import { alertManagementService } from "@/services/alertManagement";
import { alertKeys } from "./alertKeys";

// Las alertas son lo más urgente de la pantalla: se consultan con más frecuencia que los dispositivos.
const ALERTS_REFETCH_MS = 30_000;
/** Filas del panel; el total de Sin clasificar llega aparte en la respuesta. */
const PENDING_PANEL_LIMIT = 5;

/** Las Sin clasificar más recientes y cuántas hay en total. */
export function usePendingAlerts() {
    return useQuery({
        queryKey: alertKeys.pending(),
        queryFn: ({ signal }) => alertManagementService.getPending(PENDING_PANEL_LIMIT, signal),
        refetchInterval: ALERTS_REFETCH_MS,
    });
}

/** Alertas activas agrupadas por punto de medición, con el nivel mayor y si alguna titila. */
export function useActiveAlerts() {
    return useQuery({
        queryKey: alertKeys.active(),
        queryFn: ({ signal }) => alertManagementService.getActive(signal),
        refetchInterval: ALERTS_REFETCH_MS,
    });
}
