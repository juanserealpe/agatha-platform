import type { AlertLevel } from "@/services/alertManagement";

export interface SeverityStyle {
    label: string;
    /** Píldora con el nombre de la severidad. */
    badgeClass: string;
    /** Borde completo de las tarjetas de alerta. */
    borderClass: string;
    /** Anillo suave alrededor del punto de un sensor en alerta. */
    haloClass: string;
    /** Fondo tenue y opaco de los avisos (van sobre el mapa), para reconocer el nivel de un vistazo. */
    surfaceClass: string;
    /** Borde de 1 px de los avisos flotantes: el color del nivel, rebajado para que no pese. */
    hairlineClass: string;
    /**
     * Fondo de vidrio de los avisos flotantes: degradado del tono -100 al -200, traslúcido. Va con el filtro de
     * fondo de la tarjeta, que aclara y desatura el mapa (si no, el verde se mezcla con el tinte y lo ensucia).
     */
    glassClass: string;
    /** Ícono del aviso flotante, sobre su mosaico blanco. */
    iconClass: string;
    dotClass: string;
    /**
     * Fondo de los mensajes de una fase de ese nivel en la línea de tiempo (un tono sobre el -50 de §2.1,
     * que casi no se distingue del blanco), con el tono pleno y el borde al resaltarlo
     * (spec multiindicador D8, §2.1). Lo define la fase, no el tipo de mensaje.
     */
    phaseMessageClass: string;
    /** Color para capas del mapa (Leaflet no usa clases de Tailwind). */
    hex: string;
}

// Único lugar donde la escala amarillo/naranja/rojo se asocia a los niveles de alerta.
export const SEVERITY_STYLES: Record<AlertLevel, SeverityStyle> = {
    ROJO: {
        label: "Rojo",
        badgeClass: "bg-red-50 text-red-700 ring-1 ring-red-200",
        borderClass: "border-alerta-roja",
        haloClass: "ring-4 ring-alerta-roja/20",
        surfaceClass: "bg-red-50",
        hairlineClass: "border-alerta-roja/45",
        glassClass: "bg-linear-to-b from-red-100/88 to-red-200/82",
        iconClass: "text-red-600",
        dotClass: "bg-alerta-roja",
        phaseMessageClass: "bg-red-100/60 hover:bg-red-100 hover:ring-red-300",
        hex: "#ef4444",
    },
    NARANJA: {
        label: "Naranja",
        badgeClass: "bg-orange-50 text-orange-700 ring-1 ring-orange-200",
        borderClass: "border-alerta-naranja",
        haloClass: "ring-4 ring-alerta-naranja/20",
        surfaceClass: "bg-orange-50",
        hairlineClass: "border-alerta-naranja/50",
        glassClass: "bg-linear-to-b from-orange-100/88 to-orange-200/82",
        iconClass: "text-orange-600",
        dotClass: "bg-alerta-naranja",
        phaseMessageClass: "bg-orange-100/60 hover:bg-orange-100 hover:ring-orange-300",
        hex: "#f97316",
    },
    AMARILLO: {
        label: "Amarillo",
        badgeClass: "bg-amber-50 text-amber-800 ring-1 ring-amber-200",
        borderClass: "border-alerta-amarilla",
        haloClass: "ring-4 ring-alerta-amarilla/25",
        surfaceClass: "bg-amber-50",
        hairlineClass: "border-alerta-amarilla/60",
        // Un escalón más claro que naranja y rojo: la intensidad del color crece con la gravedad, y amarillo
        // es el nivel más frecuente (con -100 → -200 saturaba más que el rojo).
        glassClass: "bg-linear-to-b from-amber-50/88 to-amber-100/82",
        iconClass: "text-amber-600",
        dotClass: "bg-alerta-amarilla",
        phaseMessageClass: "bg-amber-100/60 hover:bg-amber-100 hover:ring-amber-300",
        hex: "#f59e0b",
    },
};
