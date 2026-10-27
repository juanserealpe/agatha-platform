import type { LucideIcon } from "lucide-react";
import { CircleCheck, CircleDashed, CircleDot } from "lucide-react";
import type { AlertSeverity } from "@/services/alerts";
import type {
    AlertEvent,
    AlertLabel,
    AlertLevel,
    Classification,
    ManagementOrigin,
    ManagementStatus,
} from "@/services/alertManagement";
import { OTHER_LABEL } from "@/services/alertManagement";
import type { RowAction } from "../lib/rowActions";
import { SEVERITY_STYLES, type SeverityStyle } from "./alertDisplay";

// Único lugar donde los enums de Alert Management se traducen a textos y estilos. Los componentes no
// comparan strings del backend: consultan estos catálogos.

const LEVEL_SEVERITY: Record<AlertLevel, AlertSeverity> = {
    AMARILLO: "yellow",
    NARANJA: "orange",
    ROJO: "red",
};

/** Estilo de criticidad de un nivel; reutiliza la escala amarillo/naranja/rojo de la app. */
export function levelStyle(level: AlertLevel): SeverityStyle {
    return SEVERITY_STYLES[LEVEL_SEVERITY[level]];
}

/** Orden de los niveles en filtros y leyendas: de menor a mayor criticidad. */
export const LEVEL_ORDER: readonly AlertLevel[] = ["AMARILLO", "NARANJA", "ROJO"];

export const EVENT_LABEL: Record<AlertEvent, string> = {
    FUGA: "Fuga",
    MOVIMIENTO: "Movimiento",
};

export const EVENT_ORDER: readonly AlertEvent[] = ["FUGA", "MOVIMIENTO"];

export const CLASSIFICATION_LABEL: Record<Classification, string> = {
    CONFIRMADA: "Confirmada",
    FALSA_ALERTA: "Falsa alerta",
};

export interface StatusStyle {
    label: string;
    badgeClass: string;
    /** El estado no se comunica solo con color. */
    Icon: LucideIcon;
}

// Tonos neutros a propósito: la escala amarillo/naranja/rojo queda reservada para la criticidad.
export const MANAGEMENT_STATUS_STYLES: Record<ManagementStatus, StatusStyle> = {
    SIN_CLASIFICAR: {
        label: "Sin clasificar",
        badgeClass: "bg-white text-gray-900 ring-1 ring-gray-400",
        Icon: CircleDashed,
    },
    CLASIFICADA_SIN_ETIQUETA: {
        label: "Clasificada sin etiqueta",
        badgeClass: "bg-gray-100 text-gray-700 ring-1 ring-gray-200",
        Icon: CircleDot,
    },
    CLASIFICADA_CON_ETIQUETA: {
        label: "Clasificada con etiqueta",
        badgeClass: "bg-emerald-50 text-emerald-700 ring-1 ring-emerald-200",
        Icon: CircleCheck,
    },
};

export const MANAGEMENT_STATUS_ORDER: readonly ManagementStatus[] = [
    "SIN_CLASIFICAR",
    "CLASIFICADA_SIN_ETIQUETA",
    "CLASIFICADA_CON_ETIQUETA",
];

export const LABEL_TEXT: Record<AlertLabel, string> = {
    FUGA_CONFIRMADA_CAMPO: "Fuga confirmada en campo",
    MOVIMIENTO_TIERRA: "Movimiento de tierra",
    MAQUINARIA_AJENA: "Intervención de maquinaria ajena",
    CLIMA_EXTREMO: "Condiciones climáticas extremas",
    TRANSITO_PESADO: "Tránsito de vehículo pesado",
    MANTENIMIENTO_PROGRAMADO: "Mantenimiento o intervención programada",
    OTRO: "Otro ¿Cuál?",
};

/** Texto de la etiqueta de una alerta: con OTRO se muestra el texto libre que escribió el operador. */
export function labelDisplayText(label: AlertLabel | null, otherText: string | null | undefined): string | null {
    if (label === null) return null;
    if (label === OTHER_LABEL) {
        const text = otherText?.trim();
        return text ? `Otro: ${text}` : "Otro";
    }
    return LABEL_TEXT[label];
}

export const ORIGIN_LABEL: Record<ManagementOrigin, string> = {
    WEB: "Web",
    APP_MOVIL: "App móvil",
};

/** Texto para valores ausentes en tablas y detalle. */
export const EMPTY_CELL = "—";

export const IN_PROGRESS_LABEL = "En curso";

export const ROW_ACTION_LABEL: Record<RowAction, string> = {
    classify: "Clasificar",
    label: "Agregar etiqueta",
    view: "Ver detalle",
};
