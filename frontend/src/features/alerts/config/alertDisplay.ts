import type { AlertLevel } from "@/services/alertManagement";

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

// Único lugar donde la escala amarillo/naranja/rojo se asocia a los niveles de alerta.
export const SEVERITY_STYLES: Record<AlertLevel, SeverityStyle> = {
    ROJO: {
        label: "Rojo",
        badgeClass: "bg-red-50 text-red-700 ring-1 ring-red-200",
        borderClass: "border-alerta-roja",
        dotClass: "bg-alerta-roja",
        hex: "#ef4444",
    },
    NARANJA: {
        label: "Naranja",
        badgeClass: "bg-orange-50 text-orange-700 ring-1 ring-orange-200",
        borderClass: "border-alerta-naranja",
        dotClass: "bg-alerta-naranja",
        hex: "#f97316",
    },
    AMARILLO: {
        label: "Amarillo",
        badgeClass: "bg-amber-50 text-amber-800 ring-1 ring-amber-200",
        borderClass: "border-alerta-amarilla",
        dotClass: "bg-alerta-amarilla",
        hex: "#f59e0b",
    },
};
