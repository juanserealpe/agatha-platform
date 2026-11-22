import { useCallback, useEffect, useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { alertManagementService, toAlertId } from "@/services/alertManagement";
import { findAlertGroup, type AlertGroupResult, type GroupCandidate } from "../lib/alertGroup";
import { tabSignature, tabsWithNovelty } from "../lib/alertTabs";
import { alertKeys } from "./alertKeys";
import { useAlertDetailParam } from "./useAlertDetailParam";
import { usePrefetchAlertDetail } from "./useAlertManagementDetail";

const GROUP_STALE_MS = 30_000;
// Precarga de los detalles de las otras pestañas: sin competir con la carga del modal.
const IDLE_PREFETCH_TIMEOUT_MS = 2000;

export interface AlertGroupView {
    /** En orden de inicio. Siempre incluye `seed`. */
    members: readonly GroupCandidate[];
    /** Ya llegó la consulta del grupo (antes de eso, `members` es solo `seed`). */
    ready: boolean;
}

/**
 * Alertas del mismo punto que se cruzan con `seed` (vista combinada, spec multiindicador A2), en orden de inicio.
 * Mientras carga, o si la consulta falla, el grupo es solo `seed`: la vista se ve como antes y nada se bloquea.
 * `seed` es la alerta de la pestaña abierta; si es su detalle, prevalece sobre el resumen del punto.
 *
 * La key lleva el dispositivo: el stream invalida los grupos de un punto con cada cambio de una de sus alertas, así
 * una alerta nueva aparece sola como barra y pestaña.
 */
export function useAlertGroup(seed: GroupCandidate, deviceId: number): AlertGroupView {
    const { data } = useQuery({
        queryKey: alertKeys.group(deviceId, toAlertId(seed.id)),
        queryFn: ({ signal }) =>
            findAlertGroup<GroupCandidate>(
                seed,
                async (days) => (await alertManagementService.getDeviceAlerts(deviceId, days, signal)).alerts,
                Date.now()
            ),
        staleTime: GROUP_STALE_MS,
    });

    const members = useMemo(
        () => (data ? data.members.map((member) => (member.id === seed.id ? seed : member)) : [seed]),
        [seed, data]
    );
    return { members, ready: data !== undefined };
}

/**
 * Cambiar de pestaña. El grupo es el mismo desde cualquiera de sus miembros, así que se deja en la caché de la otra
 * alerta: la franja y las pestañas no parpadean mientras se vuelve a consultar.
 */
export function useSelectGroupTab(members: readonly GroupCandidate[], deviceId: number) {
    const queryClient = useQueryClient();
    const { selectTab } = useAlertDetailParam();
    const prefetchDetail = usePrefetchAlertDetail();

    const select = useCallback(
        (alertId: number) => {
            const id = toAlertId(alertId);
            const key = alertKeys.group(deviceId, id);
            if (!queryClient.getQueryData(key)) {
                const cached: AlertGroupResult<GroupCandidate> = { members: [...members], complete: true };
                queryClient.setQueryData(key, cached);
            }
            selectTab(String(id));
        },
        [deviceId, members, queryClient, selectTab]
    );
    const prefetch = useCallback((alertId: number) => prefetchDetail(toAlertId(alertId)), [prefetchDetail]);

    return { selectTab: select, prefetchTab: prefetch };
}

/** Precarga en tiempo ocioso el detalle de las otras pestañas, para que cambiar de pestaña no espere. */
export function usePrefetchGroupDetails(members: readonly GroupCandidate[], activeId: number) {
    const prefetchDetail = usePrefetchAlertDetail();
    const others = useMemo(
        () => members.filter((member) => member.id !== activeId).map((member) => toAlertId(member.id)),
        [members, activeId]
    );
    useEffect(() => {
        if (others.length === 0) return;
        const run = () => others.forEach(prefetchDetail);
        if ("requestIdleCallback" in window) {
            const handle = window.requestIdleCallback(run, { timeout: IDLE_PREFETCH_TIMEOUT_MS });
            return () => window.cancelIdleCallback(handle);
        }
        const timer = setTimeout(run, IDLE_PREFETCH_TIMEOUT_MS);
        return () => clearTimeout(timer);
    }, [others, prefetchDetail]);
}

const NO_NOVELTY: ReadonlySet<number> = new Set();

/**
 * Punto de novedad (spec §6): pestañas no abiertas que cambiaron de estado o de nivel desde que se vieron, o que
 * aparecieron después de abrir el modal. Lo visto se fija cuando llega el grupo y se actualiza con la pestaña abierta.
 */
export function useTabNovelty({ members, ready }: AlertGroupView, activeId: number): ReadonlySet<number> {
    const [seen, setSeen] = useState<ReadonlyMap<number, string> | null>(null);
    // Patrón de React para derivar estado de props (sin efecto): lo visto se actualiza en el mismo render.
    if (ready && seen === null) {
        setSeen(new Map(members.map((member) => [member.id, tabSignature(member)])));
    }
    const active = members.find((member) => member.id === activeId);
    if (seen && active && seen.get(activeId) !== tabSignature(active)) {
        setSeen(new Map(seen).set(activeId, tabSignature(active)));
    }
    return useMemo(() => (seen ? tabsWithNovelty(members, seen, activeId) : NO_NOVELTY), [members, seen, activeId]);
}
