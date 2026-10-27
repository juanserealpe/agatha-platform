import { useQuery } from "@tanstack/react-query";
import { alertsService } from "@/services/alerts";
import { alertKeys } from "./alertKeys";

// Las alertas son lo más urgente de la pantalla: se consultan con más frecuencia que los dispositivos.
const ALERTS_REFETCH_MS = 30_000;
const DEFAULT_RECENT_LIMIT = 5;

export function useRecentAlerts(limit = DEFAULT_RECENT_LIMIT) {
    return useQuery({
        queryKey: alertKeys.recent(limit),
        queryFn: () => alertsService.getRecent(limit),
        refetchInterval: ALERTS_REFETCH_MS,
    });
}

export function usePendingAlerts() {
    return useQuery({
        queryKey: alertKeys.pending(),
        queryFn: () => alertsService.getPending(),
        refetchInterval: ALERTS_REFETCH_MS,
    });
}
