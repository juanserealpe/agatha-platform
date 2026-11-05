import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { alertManagementService } from "@/services/alertManagement";
import { sensorAlertIndicators } from "../lib/sensorAlertIndicators";
import { alertKeys } from "./alertKeys";
import { useAlertStreamStatus } from "./useAlertStream";

// Las alertas son lo más urgente de la pantalla: se consultan con más frecuencia que los dispositivos.
const ALERTS_REFETCH_MS = 30_000;
/** Filas del panel; el total de Sin clasificar llega aparte en la respuesta. */
const PENDING_PANEL_LIMIT = 5;

/** Con el stream en vivo los cambios llegan solos; el polling queda solo como respaldo cuando está caído. */
function useAlertsRefetchInterval(): number | false {
    return useAlertStreamStatus() === "live" ? false : ALERTS_REFETCH_MS;
}

/** Las Sin clasificar más recientes y cuántas hay en total. */
export function usePendingAlerts() {
    return useQuery({
        queryKey: alertKeys.pending(),
        queryFn: ({ signal }) => alertManagementService.getPending(PENDING_PANEL_LIMIT, signal),
        refetchInterval: useAlertsRefetchInterval(),
    });
}

/** Alertas activas agrupadas por punto de medición, con el nivel mayor y si alguna titila. */
export function useActiveAlerts() {
    return useQuery({
        queryKey: alertKeys.active(),
        queryFn: ({ signal }) => alertManagementService.getActive(signal),
        refetchInterval: useAlertsRefetchInterval(),
    });
}

/** Color de alerta de cada sensor en la lista, desde las mismas alertas activas que pintan el mapa. */
export function useSensorAlertIndicators() {
    const { data } = useActiveAlerts();
    return useMemo(() => sensorAlertIndicators(data), [data]);
}
