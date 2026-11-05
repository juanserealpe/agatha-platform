import type { AlertId, AlertItem } from "@/services/alertManagement";
import type { DemoAlertStore } from "./alertStore";
import { DEMO_POINTS } from "./demoData";

// A qué alerta o punto se aplican los botones sueltos del panel. Elige igual que lo haría el presentador:
// la alerta activa más reciente que admita la acción, y para una alerta nueva, un punto libre.

/** Los puntos 2 y 3 primero, para no ocupar el Punto 1 del escenario guiado. */
const NEW_ALERT_POINT_ORDER = [DEMO_POINTS.punto2.id, DEMO_POINTS.punto3.id, DEMO_POINTS.punto1.id];

function activeAlertsNewestFirst(store: DemoAlertStore): AlertItem[] {
    return store
        .getActive()
        .devices.flatMap((device) => device.alerts)
        .sort((a, b) => Date.parse(b.started_at) - Date.parse(a.started_at) || b.id - a.id);
}

/** Primer punto sin alerta activa; null si los tres tienen una (el backend no abre dos en el mismo punto). */
export function pointForNewAlert(store: DemoAlertStore): number | null {
    return NEW_ALERT_POINT_ORDER.find((pointId) => store.activeAlertId(pointId) === null) ?? null;
}

/** Alerta activa más reciente que todavía puede subir de nivel. */
export function alertToRaise(store: DemoAlertStore): AlertId | null {
    return activeAlertsNewestFirst(store).find((alert) => alert.current_level !== "ROJO")?.id ?? null;
}

/** Alerta activa más reciente que se puede confirmar. */
export function alertToClassify(store: DemoAlertStore): AlertId | null {
    return activeAlertsNewestFirst(store).find((alert) => alert.available_actions.includes("CONFIRM"))?.id ?? null;
}
