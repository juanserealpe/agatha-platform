import type { AlertEvent, AlertLevel, LevelOpenedBy, MessageKind } from "@/services/alertManagement";

// Textos en lenguaje claro para quien monitorea sin ser experto. Se ajustan aquí, sin tocar componentes.
// El indicador del backend es un PUNTAJE DE ANOMALÍA, no una probabilidad calibrada de fuga: ningún texto
// debe presentarlo como porcentaje ni como "probabilidad".

export const EVENT_EXPLANATION: Record<AlertEvent, { title: string; description: string }> = {
    FUGA: {
        title: "Posible fuga",
        description: "El sensor acústico escuchó en el ducto un sonido distinto al habitual, que puede indicar un escape de gas.",
    },
    MOVIMIENTO: {
        title: "Posible movimiento del terreno",
        description: "El sensor registró un movimiento o inclinación del terreno distinto al habitual en este punto.",
    },
};

/** Qué tan lejos está la medición de lo normal, según la criticidad alcanzada. */
export const ANOMALY_PHRASE: Record<AlertEvent, Record<AlertLevel, string>> = {
    FUGA: {
        AMARILLO: "El sonido del ducto en este punto está algo por encima de su comportamiento normal.",
        NARANJA: "El sonido del ducto en este punto está claramente por encima de su comportamiento normal.",
        ROJO: "El sonido del ducto en este punto está muy por encima de su comportamiento normal.",
    },
    MOVIMIENTO: {
        AMARILLO: "El terreno en este punto se mueve algo más de lo normal.",
        NARANJA: "El terreno en este punto se mueve claramente más de lo normal.",
        ROJO: "El terreno en este punto se mueve mucho más de lo normal.",
    },
};

export const ANOMALY_SCORE_NOTE =
    "El nivel de anomalía compara la medición con el comportamiento habitual de este punto: entre más alto, más se aleja de lo normal. No es una probabilidad de fuga.";

export const OPENED_BY_LABEL: Record<LevelOpenedBy, string> = {
    OPEN: "Apertura",
    UP: "Subió",
    DOWN: "Bajó",
    REACTIVATION: "Reactivación",
};

export const MESSAGE_KIND_LABEL: Record<MessageKind, string> = {
    NOTIFICACION: "Notificación",
    ACTUALIZACION: "Actualización",
    RESUMEN: "Resumen",
    FIN: "Fin",
    GESTION: "Gestión",
};

const ANOMALY_FORMATTER = new Intl.NumberFormat("es-CO", { maximumFractionDigits: 2 });

/** Puntaje de anomalía como número legible ("3,25"); nunca como porcentaje. */
export function formatAnomalyScore(value: number): string {
    return Number.isFinite(value) ? ANOMALY_FORMATTER.format(value) : "—";
}
