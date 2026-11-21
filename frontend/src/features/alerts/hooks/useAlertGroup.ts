import { useCallback, useMemo } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { alertManagementService, toAlertId, type AlertDetail } from "@/services/alertManagement";
import { findAlertGroup, type AlertGroupResult, type GroupCandidate } from "../lib/alertGroup";
import { alertKeys } from "./alertKeys";
import { useAlertDetailParam } from "./useAlertDetailParam";
import { usePrefetchAlertDetail } from "./useAlertManagementDetail";

const GROUP_STALE_MS = 30_000;

/**
 * Alertas del mismo punto que se cruzan con la abierta (vista combinada, spec multiindicador A2), en orden de
 * inicio. Mientras carga, o si la consulta falla, el grupo es solo la alerta abierta: la vista se ve como antes
 * y nada se bloquea. La alerta abierta siempre usa su detalle, que es lo más reciente.
 */
export function useAlertGroup(detail: AlertDetail): readonly GroupCandidate[] {
    const { data } = useQuery({
        queryKey: alertKeys.group(detail.id),
        queryFn: ({ signal }) =>
            findAlertGroup<GroupCandidate>(
                detail,
                async (days) => (await alertManagementService.getDeviceAlerts(detail.device.id, days, signal)).alerts,
                Date.now()
            ),
        staleTime: GROUP_STALE_MS,
    });

    return useMemo(
        () => (data ? data.members.map((member) => (member.id === detail.id ? detail : member)) : [detail]),
        [detail, data]
    );
}

/**
 * Abrir otra alerta del grupo desde la franja. El grupo es el mismo desde cualquiera de sus miembros, así que se deja
 * en la caché de la otra alerta: la franja no parpadea de N barras a 1 mientras se vuelve a consultar.
 */
export function useOpenGroupMember(group: readonly GroupCandidate[]) {
    const queryClient = useQueryClient();
    const { open } = useAlertDetailParam();
    const prefetchDetail = usePrefetchAlertDetail();

    const openMember = useCallback(
        (alertId: number) => {
            const id = toAlertId(alertId);
            const cached: AlertGroupResult<GroupCandidate> = { members: [...group], complete: true };
            queryClient.setQueryData(alertKeys.group(id), cached);
            open(String(id));
        },
        [group, open, queryClient]
    );
    const prefetchMember = useCallback((alertId: number) => prefetchDetail(toAlertId(alertId)), [prefetchDetail]);

    return { openMember, prefetchMember };
}
