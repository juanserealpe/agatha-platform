import type { AlertSeverity, AlertStatus, AlertType } from "@/services/alerts";

export interface SeverityStyle {
    label: string;
    /** Píldora con el nombre de la severidad. */
    badgeClass: string;
    /** Borde izquierdo de las tarjetas de alerta. */
    accentClass: string;
    dotClass: string;
    /** Color para capas del mapa (Leaflet no usa clases de Tailwind). */
    hex: string;
}

// Único lugar donde la escala amarillo/naranja/rojo se asocia a severidad.
export const SEVERITY_STYLES: Record<AlertSeverity, SeverityStyle> = {
    red: {
        label: "Rojo",
        badgeClass: "bg-red-50 text-red-700 ring-1 ring-red-200",
        accentClass: "border-l-alerta-roja",
        dotClass: "bg-alerta-roja",
        hex: "#ef4444",
    },
    orange: {
        label: "Naranja",
        badgeClass: "bg-orange-50 text-orange-700 ring-1 ring-orange-200",
        accentClass: "border-l-alerta-naranja",
        dotClass: "bg-alerta-naranja",
        hex: "#f97316",
    },
    yellow: {
        label: "Amarillo",
        badgeClass: "bg-amber-50 text-amber-800 ring-1 ring-amber-200",
        accentClass: "border-l-alerta-amarilla",
        dotClass: "bg-alerta-amarilla",
        hex: "#f59e0b",
    },
};

/** Orden de prioridad: menor número, más grave. */
export const SEVERITY_RANK: Record<AlertSeverity, number> = { red: 0, orange: 1, yellow: 2 };

export const ALERT_TYPE_LABEL: Record<AlertType, string> = {
    leak: "Fuga",
    movement: "Movimiento",
};

export const ALERT_STATUS_LABEL: Record<AlertStatus, string> = {
    unclassified: "Sin clasificar",
    in_progress: "En curso",
    confirmed: "Confirmada",
    dismissed: "Falsa alerta",
};
