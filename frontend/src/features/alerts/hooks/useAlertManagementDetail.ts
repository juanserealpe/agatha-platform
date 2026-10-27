import { useQuery } from "@tanstack/react-query";
import { alertManagementService, toAlertId, type AlertId } from "@/services/alertManagement";
import { alertKeys } from "./alertKeys";

const DETAIL_STALE_MS = 30_000;
// Solo completa la query key mientras no hay alerta; la consulta está deshabilitada y nunca se pide.
const NO_ALERT = toAlertId(0);

/** Detalle completo de una alerta. Solo consulta cuando hay un id, es decir, cuando el modal está abierto. */
export function useAlertManagementDetail(id: AlertId | null) {
    return useQuery({
        queryKey: alertKeys.management(id ?? NO_ALERT),
        queryFn: ({ signal }) => alertManagementService.getDetail(id!, signal),
        enabled: id !== null,
        staleTime: DETAIL_STALE_MS,
    });
}
