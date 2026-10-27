import { useCallback } from "react";
import { queryOptions, useQuery, useQueryClient } from "@tanstack/react-query";
import { alertManagementService, toAlertId, type AlertId } from "@/services/alertManagement";
import { alertKeys } from "./alertKeys";

const DETAIL_STALE_MS = 30_000;
// Solo completa la query key mientras no hay alerta; la consulta está deshabilitada y nunca se pide.
const NO_ALERT = toAlertId(0);

function detailQuery(id: AlertId) {
    return queryOptions({
        queryKey: alertKeys.management(id),
        queryFn: ({ signal }) => alertManagementService.getDetail(id, signal),
        staleTime: DETAIL_STALE_MS,
    });
}

/** Detalle completo de una alerta. Solo consulta cuando hay un id, es decir, cuando el modal está abierto. */
export function useAlertManagementDetail(id: AlertId | null) {
    return useQuery({ ...detailQuery(id ?? NO_ALERT), enabled: id !== null });
}

/** Precarga el detalle cuando el usuario se acerca a una acción, para que el modal abra sin espera. */
export function usePrefetchAlertDetail() {
    const queryClient = useQueryClient();
    return useCallback((id: AlertId) => void queryClient.prefetchQuery(detailQuery(id)), [queryClient]);
}
