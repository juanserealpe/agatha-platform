// Modelo de dominio de alertas.
// Propuesta del frontend: el backend aún no expone alertas. Ajustar al contrato cuando se defina.
// Pendiente: el aviso gris de "indicador no calculable" (pérdida de datos) no es una severidad;
// puede coexistir con una alerta, así que se modelará aparte cuando el backend defina cómo lo reporta.

export type AlertSeverity = "red" | "orange" | "yellow";

export type AlertType = "leak" | "movement";

export type AlertStatus =
    | "unclassified" // pendiente de que un operador la clasifique
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
