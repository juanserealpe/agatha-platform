// Modelo de dominio de alertas.
// Propuesta del frontend: el backend aún no expone alertas. Ajustar al contrato cuando se defina.

export type AlertSeverity = "red" | "orange" | "yellow";

export type AlertType = "leak" | "movement";

export type AlertStatus =
    | "unclassified" // pendiente de que un operador la clasifique
    | "in_progress" // en atención
    | "confirmed" // evento real
    | "dismissed"; // falsa alerta

export interface Alert {
    id: string;
    severity: AlertSeverity;
    type: AlertType;
    status: AlertStatus;
    deviceId: number;
    /** Posición sobre el ducto en metros; null si el evento no está asociado a un punto. */
    pkMeters: number | null;
    /** ISO 8601. */
    detectedAt: string;
}
