import type { AlertLevel } from "@/services/alertManagement";

/** Color del punto en el mapa: normal, el nivel de su alerta o no disponible (gris). */
export type NodeTone = "normal" | AlertLevel | "unavailable";

export interface NodeState {
    tone: NodeTone;
    /** Aviso (sin comunicación) superpuesto a una alerta: el punto conserva el color de la alerta. */
    hasWarning: boolean;
    /** La alerta roja titila mientras siga sin clasificar. */
    isBlinking: boolean;
}

/** Resumen de las alertas activas del punto, ya calculado por el backend (GET /api/alerts/active). */
export interface NodeAlerts {
    /** La mayor criticidad entre sus alertas activas. */
    level: AlertLevel;
    /** Alguna está en ROJO sin clasificar. */
    blinking: boolean;
}

export interface NodeInput {
    /** false si el punto no tiene dispositivo enlazado (no operativo). */
    isLinked: boolean;
    /** El dispositivo dejó de comunicarse. */
    isOffline: boolean;
    /** null si el punto no tiene alertas activas. */
    alerts: NodeAlerts | null;
}

export function getNodeState({ isLinked, isOffline, alerts }: NodeInput): NodeState {
    if (!isLinked) return { tone: "unavailable", hasWarning: false, isBlinking: false };

    if (!alerts) {
        return { tone: isOffline ? "unavailable" : "normal", hasWarning: false, isBlinking: false };
    }

    return { tone: alerts.level, hasWarning: isOffline, isBlinking: alerts.blinking };
}
