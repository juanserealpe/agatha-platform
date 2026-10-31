import { useEffect, useSyncExternalStore } from "react";
import type { AlertId } from "@/services/alertManagement";
import { dismissAlert, ingestAlerts, INITIAL_NEW_ALERTS_STATE, type NewAlertsState } from "../lib/newAlerts";
import { usePendingAlerts } from "./useAlerts";

// Estado a nivel de módulo: sobrevive al cambio de pestaña, así una alerta que llega mientras el
// usuario está en Historial se notifica al volver al mapa (en vez de entrar en la línea base).
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
 * Alertas que llegaron después de abrir la pantalla y que el usuario aún no ha visto ni descartado.
 * Por ahora se detectan como ids nuevos en la lista de Sin clasificar; una subida de nivel conserva el id
 * y no se avisa hasta que el aviso se alimente del stream.
 */
export function useNewAlerts() {
    const { data, isSuccess } = usePendingAlerts();
    const current = useSyncExternalStore(subscribe, () => state);

    useEffect(() => {
        if (isSuccess) setState(ingestAlerts(state, data.items));
    }, [data, isSuccess]);

    return {
        unseen: current.unseen,
        dismiss: (alertId: AlertId) => setState(dismissAlert(state, alertId)),
    };
}
