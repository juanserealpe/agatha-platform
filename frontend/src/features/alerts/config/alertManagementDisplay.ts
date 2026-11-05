import type { LucideIcon } from "lucide-react";
import { CircleCheck, CircleDashed, CircleDot } from "lucide-react";
import type {
    AlertEvent,
    AlertItem,
    AlertLabel,
    AlertLevel,
    Classification,
    ManagementOrigin,
    ManagementStatus,
} from "@/services/alertManagement";
import { OTHER_LABEL } from "@/services/alertManagement";
import type { NotificationCode } from "@/services/realtime";
import type { RowAction } from "../lib/rowActions";
import { SEVERITY_STYLES, type SeverityStyle } from "./alertDisplay";

// Único lugar donde los enums de Alert Management se traducen a textos y estilos. Los componentes no
// comparan strings del backend: consultan estos catálogos.

/** Estilo de criticidad de un nivel; reutiliza la escala amarillo/naranja/rojo de la app. */
export function levelStyle(level: AlertLevel): SeverityStyle {
    return SEVERITY_STYLES[level];
}

/** Orden de los niveles en filtros y leyendas: de menor a mayor criticidad. */
export const LEVEL_ORDER: readonly AlertLevel[] = ["AMARILLO", "NARANJA", "ROJO"];

/** Nombre de un sensor o punto que está en ese nivel (lista de sensores, leyenda del mapa). */
export const LEVEL_ALERT_LABEL: Record<AlertLevel, string> = {
    AMARILLO: "Alerta amarilla",
    NARANJA: "Alerta naranja",
    ROJO: "Alerta roja",
};

/** Nivel de una alerta en listas y avisos: el actual o, si ya finalizó, el máximo que alcanzó. */
export function displayLevel(alert: Pick<AlertItem, "current_level" | "max_level">): AlertLevel {
    return alert.current_level ?? alert.max_level;
}

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

/** Encabezado del banner según el aviso del catálogo (`alert.notification`). */
export const NOTICE_KIND_LABEL: Record<NotificationCode, string> = {
    OPEN: "Nueva alerta",
    NEW_AFTER_CLASSIFIED: "Nueva alerta",
    UP_NARANJA_PERSIST: "Subió de nivel",
    UP_NARANJA_FAST: "Subió de nivel",
    UP_ROJO: "Subió de nivel",
    REACTIVATED: "Alerta reactivada",
};

/** Aviso de una hora que llegó atrasada: se muestra igual, marcado para que no se ignore. */
export const LATE_NOTICE_LABEL = "Atrasado";

export const ROW_ACTION_LABEL: Record<RowAction, string> = {
    classify: "Clasificar",
    label: "Agregar etiqueta",
    view: "Ver detalle",
};

// Chip de nivel seleccionado: conserva el color de su criticidad (con check) en vez del oscuro genérico.
// Clases completas y literales para que Tailwind las genere.
export const LEVEL_CHIP_SELECTED_CLASS: Record<AlertLevel, string> = {
    AMARILLO: "aria-pressed:border-amber-300 aria-pressed:bg-amber-50 aria-pressed:text-amber-800 aria-pressed:hover:bg-amber-100",
    NARANJA: "aria-pressed:border-orange-300 aria-pressed:bg-orange-50 aria-pressed:text-orange-700 aria-pressed:hover:bg-orange-100",
    ROJO: "aria-pressed:border-red-300 aria-pressed:bg-red-50 aria-pressed:text-red-700 aria-pressed:hover:bg-red-100",
};
