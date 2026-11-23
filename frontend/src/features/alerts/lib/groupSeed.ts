import type { ActiveResponse, AlertId, AlertItem, AlertSearchResponse, PendingResponse } from "@/services/alertManagement";
import type { GroupCandidate } from "./alertGroup";

/** Listas de alertas que ya están en la caché (panel, mapa e historial). */
export interface ListedAlerts {
    histories: readonly (AlertSearchResponse | undefined)[];
    pending: PendingResponse | undefined;
    active: ActiveResponse | undefined;
}

/** La alerta tal como aparece en alguna lista ya consultada, si está en alguna. */
export function findListedAlert({ histories, pending, active }: ListedAlerts, id: AlertId): AlertItem | undefined {
    const lists = [
        pending?.items,
        active?.devices.flatMap((device) => device.alerts),
        ...histories.map((page) => page?.items),
    ];
    for (const list of lists) {
        const found = list?.find((item) => item.id === id);
        if (found) return found;
    }
    return undefined;
}

/**
 * Semilla del grupo a partir de la fila de una lista, para buscar el grupo SIN esperar el detalle: la búsqueda solo
 * usa el rango de la semilla (inicio, fin y estado, iguales en la fila y en el detalle). La fila no trae los tramos,
 * pero no hacen falta: al mostrar el grupo, la semilla se reemplaza por el detalle (useAlertGroup).
 */
export function groupSeedFromItem(item: AlertItem): GroupCandidate {
    return {
        id: item.id,
        code: item.code,
        event: item.event,
        state: item.state,
        started_at: item.started_at,
        ended_at: item.ended_at,
        max_level: item.max_level,
        classification: item.classification,
        levels: [],
    };
}
