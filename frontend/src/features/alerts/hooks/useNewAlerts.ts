import { useSyncExternalStore } from "react";
import type { AlertNotificationEvent } from "@/services/realtime";
import { dismissNotice, ingestNotification, INITIAL_NEW_ALERTS_STATE, type NewAlertsState } from "../lib/newAlerts";

// Estado a nivel de módulo: sobrevive al cambio de pestaña, así un aviso que llega mientras el usuario está
// en Historial se sigue mostrando al volver al mapa.
let state: NewAlertsState = INITIAL_NEW_ALERTS_STATE;
const listeners = new Set<() => void>();

function setState(next: NewAlertsState) {
    if (next === state) return;
    state = next;
    listeners.forEach((notify) => notify());
}

function subscribe(listener: () => void) {
    listeners.add(listener);
    return () => listeners.delete(listener);
}

/**
 * Entrada de los avisos `alert.notification` del stream (la llama useAlertStream). Devuelve false si el aviso
 * ya se había recibido (reenvío tras una reconexión), para no repetir notificación ni sonido.
 */
export function receiveNotification(notice: AlertNotificationEvent): boolean {
    const previous = state;
    setState(ingestNotification(previous, notice));
    // ingestNotification devuelve el mismo estado solo si el message_id ya se había visto.
    return state !== previous;
}

/** Quita el banner de la alerta (descartado, o ya abierto desde el banner o la notificación del navegador). */
export function dismissNotification(alertCode: string) {
    setState(dismissNotice(state, alertCode));
}

/** Avisos de alerta nueva o de subida de nivel que el usuario aún no ha visto ni descartado. */
export function useNewAlerts() {
    const current = useSyncExternalStore(subscribe, () => state);
    return { notices: current.notices, dismiss: dismissNotification };
}
