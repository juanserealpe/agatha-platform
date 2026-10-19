import type { Alert } from "@/services/alerts";
import { SEVERITY_RANK } from "../config/alertDisplay";

export interface NewAlertsState {
    /** Ids ya conocidos; null hasta recibir la primera lista (que no genera avisos). */
    seenIds: ReadonlySet<string> | null;
    /** Alertas nuevas pendientes de que el usuario las vea o descarte. */
    unseen: Alert[];
}

export const INITIAL_NEW_ALERTS_STATE: NewAlertsState = { seenIds: null, unseen: [] };

/**
 * Incorpora la lista más reciente de alertas. La primera lista solo fija la línea base
 * (lo que ya existía al abrir la pantalla no se notifica); después, todo id desconocido es nuevo.
 */
export function ingestAlerts(state: NewAlertsState, alerts: Alert[]): NewAlertsState {
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

export function dismissAlert(state: NewAlertsState, alertId: string): NewAlertsState {
    return { ...state, unseen: state.unseen.filter((alert) => alert.id !== alertId) };
}

/** La alerta a mostrar primero: la más grave y, entre iguales, la más reciente. */
export function pickHeadline(alerts: Alert[]): Alert | null {
    return (
        [...alerts].sort(
            (a, b) =>
                SEVERITY_RANK[a.severity] - SEVERITY_RANK[b.severity] ||
                new Date(b.detectedAt).getTime() - new Date(a.detectedAt).getTime()
        )[0] ?? null
    );
}
