import type { ActiveResponse, AlertItem } from "@/services/alertManagement";
import { displayLevel, LEVEL_ORDER } from "../config/alertManagementDisplay";

/** Orden de atención: mayor nivel primero; entre iguales, la que titila y después la que inició más tarde. */
export function compareByPriority(a: AlertItem, b: AlertItem): number {
    return (
        LEVEL_ORDER.indexOf(displayLevel(b)) - LEVEL_ORDER.indexOf(displayLevel(a)) ||
        Number(b.blinking) - Number(a.blinking) ||
        new Date(b.started_at).getTime() - new Date(a.started_at).getTime()
    );
}

/** Todas las alertas activas de todos los puntos, de la más urgente a la menos urgente (pestaña "Activas"). */
export function activeAlertsByPriority(data: ActiveResponse | undefined): AlertItem[] {
    return (data?.devices ?? []).flatMap((device) => device.alerts).sort(compareByPriority);
}

/** La alerta a mostrar primero (p. ej. al abrir un punto del mapa); null si no hay ninguna. */
export function mostUrgentAlert(alerts: readonly AlertItem[]): AlertItem | null {
    return [...alerts].sort(compareByPriority)[0] ?? null;
}
