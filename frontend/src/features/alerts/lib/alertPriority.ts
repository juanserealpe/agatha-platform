import type { AlertItem } from "@/services/alertManagement";
import { displayLevel, LEVEL_ORDER } from "../config/alertManagementDisplay";

/** Orden de atención: mayor nivel primero; entre iguales, la que titila y después la que inició más tarde. */
export function compareByPriority(a: AlertItem, b: AlertItem): number {
    return (
        LEVEL_ORDER.indexOf(displayLevel(b)) - LEVEL_ORDER.indexOf(displayLevel(a)) ||
        Number(b.blinking) - Number(a.blinking) ||
        new Date(b.started_at).getTime() - new Date(a.started_at).getTime()
    );
}

/** La alerta a mostrar primero (aviso de alerta nueva, punto del mapa); null si no hay ninguna. */
export function mostUrgentAlert(alerts: readonly AlertItem[]): AlertItem | null {
    return [...alerts].sort(compareByPriority)[0] ?? null;
}
