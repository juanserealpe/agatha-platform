import type { AlertSeverity, AlertStatus, AlertType } from "@/services/alerts";

export interface SeverityStyle {
    label: string;
    /** Píldora con el nombre de la severidad. */
    badgeClass: string;
    /** Borde completo de las tarjetas de alerta. */
    borderClass: string;
    dotClass: string;
    /** Color para capas del mapa (Leaflet no usa clases de Tailwind). */
    hex: string;
}

// Único lugar donde la escala amarillo/naranja/rojo se asocia a severidad.
export const SEVERITY_STYLES: Record<AlertSeverity, SeverityStyle> = {
    red: {
        label: "Rojo",
        badgeClass: "bg-red-50 text-red-700 ring-1 ring-red-200",
        borderClass: "border-alerta-roja",
        dotClass: "bg-alerta-roja",
        hex: "#ef4444",
    },
    orange: {
        label: "Naranja",
        badgeClass: "bg-orange-50 text-orange-700 ring-1 ring-orange-200",
        borderClass: "border-alerta-naranja",
        dotClass: "bg-alerta-naranja",
        hex: "#f97316",
    },
    yellow: {
        label: "Amarillo",
        badgeClass: "bg-amber-50 text-amber-800 ring-1 ring-amber-200",
        borderClass: "border-alerta-amarilla",
        dotClass: "bg-alerta-amarilla",
        hex: "#f59e0b",
    },
};

/** Pulsa (resplandor) la alerta que exige atención inmediata: roja y sin clasificar, igual que en el mapa. */
export function shouldPulse(alert: { severity: AlertSeverity; status: AlertStatus }): boolean {
    return alert.severity === "red" && alert.status === "unclassified";
}

/** Orden de prioridad: menor número, más grave. */
export const SEVERITY_RANK: Record<AlertSeverity, number> = { red: 0, orange: 1, yellow: 2 };

export const ALERT_TYPE_LABEL: Record<AlertType, string> = {
    leak: "Fuga",
    movement: "Movimiento",
};

export const ALERT_STATUS_LABEL: Record<AlertStatus, string> = {
    unclassified: "Sin clasificar",
    confirmed: "Confirmada",
    dismissed: "Falsa alerta",
};
