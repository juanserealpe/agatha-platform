import type { AlertId, AlertItem } from "@/services/alertManagement";

export interface NewAlertsState {
    /** Ids ya conocidos; null hasta recibir la primera lista (que no genera avisos). */
    seenIds: ReadonlySet<AlertId> | null;
    /** Alertas nuevas pendientes de que el usuario las vea o descarte. */
    unseen: AlertItem[];
}

export const INITIAL_NEW_ALERTS_STATE: NewAlertsState = { seenIds: null, unseen: [] };

/**
 * Incorpora la lista más reciente de alertas. La primera lista solo fija la línea base
 * (lo que ya existía al abrir la pantalla no se notifica); después, todo id desconocido es nuevo.
 */
export function ingestAlerts(state: NewAlertsState, alerts: AlertItem[]): NewAlertsState {
    if (state.seenIds === null) {
        return { seenIds: new Set(alerts.map((alert) => alert.id)), unseen: [] };
    }

    const fresh = alerts.filter((alert) => !state.seenIds!.has(alert.id));
    if (fresh.length === 0) return state;

    return {
        seenIds: new Set([...state.seenIds, ...fresh.map((alert) => alert.id)]),
        unseen: [...state.unseen, ...fresh],
    };
}

export function dismissAlert(state: NewAlertsState, alertId: AlertId): NewAlertsState {
    return { ...state, unseen: state.unseen.filter((alert) => alert.id !== alertId) };
}
