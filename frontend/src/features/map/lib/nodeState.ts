import type { Alert, AlertSeverity } from "@/services/alerts";

/** Color del punto en el mapa: normal, una severidad de alerta o no disponible (gris). */
export type NodeTone = "normal" | AlertSeverity | "unavailable";

export interface NodeState {
    tone: NodeTone;
    /** Aviso (sin comunicación) superpuesto a una alerta: el punto conserva el color de la alerta. */
    hasWarning: boolean;
    /** La alerta roja titila mientras siga sin clasificar. */
    isBlinking: boolean;
}

export interface NodeInput {
    /** false si el punto no tiene dispositivo enlazado (no operativo). */
    isLinked: boolean;
    /** El dispositivo dejó de comunicarse. */
    isOffline: boolean;
    /** Alertas activas del dispositivo. */
    alerts: Alert[];
}

const SEVERITY_ORDER: AlertSeverity[] = ["red", "orange", "yellow"];

/** La alerta vigente más grave (las falsas alertas no cuentan). */
export function pickMostSevereAlert(alerts: Alert[]): Alert | null {
    const active = alerts.filter((alert) => alert.status !== "dismissed");
    for (const severity of SEVERITY_ORDER) {
        const match = active.find((alert) => alert.severity === severity);
        if (match) return match;
    }
    return null;
}

export function getNodeState({ isLinked, isOffline, alerts }: NodeInput): NodeState {
    if (!isLinked) return { tone: "unavailable", hasWarning: false, isBlinking: false };

    const alert = pickMostSevereAlert(alerts);
    if (!alert) {
        return { tone: isOffline ? "unavailable" : "normal", hasWarning: false, isBlinking: false };
    }

    return {
        tone: alert.severity,
        hasWarning: isOffline,
        isBlinking: alert.severity === "red" && alert.status === "unclassified",
    };
}
