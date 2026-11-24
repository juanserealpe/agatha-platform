import type { AlertEvent, AlertLevel, LeakSignature, LevelOpenedBy, MessageKind } from "@/services/alertManagement";

// Textos en lenguaje claro para quien monitorea sin ser experto. Se ajustan aquí, sin tocar componentes.
// El indicador del backend es un PUNTAJE DE ANOMALÍA, no una probabilidad calibrada de fuga: ningún texto
// debe presentarlo como porcentaje ni como "probabilidad".

export interface EventExplanation {
    title: string;
    description: string;
    /** Qué tan lejos está la medición de lo normal según el nivel alcanzado; solo los niveles que el evento emite. */
    levelPhrase: Partial<Record<AlertLevel, string>>;
    /**
     * El backend calcula un puntaje de anomalía para este evento. Movimiento no lo tiene: el detalle lo envía en 0
     * y sus datos propios (pitch/roll, temperatura) llegan en la entrega 2 (spec multiindicador A7).
     */
    hasAnomalyScore: boolean;
}

export const EVENT_EXPLANATION: Record<AlertEvent, EventExplanation> = {
    FUGA: {
        title: "Anomalía acústica",
        description:
            "El sensor acústico escuchó en el ducto un sonido distinto al habitual en este punto. No siempre es una fuga: otras fuentes de ruido también lo producen.",
        levelPhrase: {
            AMARILLO: "El sonido del ducto en este punto está algo por encima de su comportamiento normal.",
            NARANJA: "El sonido del ducto en este punto está claramente por encima de su comportamiento normal.",
            ROJO: "El sonido del ducto en este punto está muy por encima de su comportamiento normal.",
        },
        hasAnomalyScore: true,
    },
    // Describe lo que midió el sensor, sin afirmar la causa ni hablar de maniobras del gasoducto. El terreno se nombra
    // solo como posibilidad a verificar en campo, como DETECCION_ANOMALIAS §1 del backend. Desde motion-v2 (catálogo v8) el nivel sale de la tabla de lo normal
    // de cada sensor y puede llegar a Naranja o Rojo; también alerta el cambio lento acumulado (MOV_OPEN/ACUMULADO).
    MOVIMIENTO: {
        title: "Cambio de inclinación del sensor",
        description:
            "El sensor de este punto cambió de inclinación (pitch y roll) más de lo normal para este sensor: de golpe, en pocas horas o poco a poco durante varios días, o quedó en otra posición. Puede deberse a un movimiento del terreno, como un deslizamiento, o del propio sensor; solo una verificación en campo lo confirma.",
        levelPhrase: {
            AMARILLO: "La inclinación del sensor cambió algo más de lo normal en este punto.",
            NARANJA: "La inclinación del sensor cambió claramente más de lo normal en este punto.",
            ROJO: "La inclinación del sensor cambió mucho más de lo normal en este punto.",
        },
        hasAnomalyScore: false,
    },
};

/**
 * Rótulo del chip junto a cada puntaje (spec multiindicador §9.2). El chip es el nivel en que estaba la alerta, no
 * el nivel del valor: un Rojo sin clasificar sigue en Rojo aunque la última hora esté por debajo de lo normal.
 */
export const SCORE_LEVEL_CAPTION = {
    opening: "La alerta abrió en",
    lastInProgress: "La alerta sigue en",
    lastFinished: "La alerta terminó en",
} as const;

/** Meta de los mensajes `TEMP_*`: contexto del sensor común a fuga y movimiento (spec multiindicador §7.1). */
export const SENSOR_EVENT_LABEL = "Evento del sensor";

/** Duración del grupo (D6). Con una alerta sola es la de esa alerta y va en su línea de tiempo. */
export const GENERAL_DURATION_LABEL = "Duración general";
/** Con varias alertas en la vista, la línea de tiempo muestra la duración de la abierta con este rótulo. */
export const ALERT_DURATION_LABEL = "Duración de esta alerta";
/** Nivel del grupo (D4): el actual más alto mientras alguna siga en curso; el máximo alcanzado al finalizar. */
export const GENERAL_LEVEL_LABEL = "Nivel general";

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

/** Al pasar el cursor: distingue un cambio que se avisó de una actualización que solo queda registrada. */
export const MESSAGE_KIND_DESCRIPTION: Record<MessageKind, string> = {
    // También son NOTIFICACION los TEMP_STABILIZED / TEMP_VARIES_AGAIN, que no abren ni suben (MOVIMIENTO_ALERTAS §5.3).
    NOTIFICACION:
        "Cambio importante que se avisó al operador, por ejemplo: la alerta abrió, subió de nivel o la temperatura de la tubería cambió de comportamiento.",
    ACTUALIZACION: "Actualización del estado de la alerta. Queda registrada, sin aviso.",
    RESUMEN: "Resumen de la alerta.",
    FIN: "La medición volvió a lo normal y la alerta terminó.",
    GESTION: "Acción del operador: clasificación o etiqueta.",
};

const PERCENT_FORMATTER = new Intl.NumberFormat("es-CO", { maximumFractionDigits: 1 });

/**
 * Firma de fuga (planitud espectral) en lenguaje claro: el % de mediciones con el patrón de sonido típico de una
 * fuga, con el mismo vocabulario del catálogo v9 del backend. null si no hay dato para mostrar. Como el catálogo,
 * no afirma que sea una fuga: solo si el sonido se parece.
 */
export function leakSignatureText(signature: LeakSignature | null | undefined): { title: string; text: string } | null {
    if (!signature || signature.d_pl === null) return null;
    const value = `${PERCENT_FORMATTER.format(signature.d_pl)} %`;
    const min = `${PERCENT_FORMATTER.format(signature.d_pl_min)} %`;
    if (signature.confirmed) {
        return {
            title: "El sonido se parece al de una fuga",
            text: `En las últimas 24 h, el ${value} de las mediciones tuvo el patrón de sonido típico de una fuga (a partir del ${min} se considera parecido).`,
        };
    }
    return {
        title: "El sonido no se parece al de una fuga",
        text: `En las últimas 24 h, solo el ${value} de las mediciones tuvo el patrón de sonido típico de una fuga; se necesita al menos el ${min}. El sonido es anómalo, pero no tiene ese patrón.`,
    };
}

const ANOMALY_FORMATTER = new Intl.NumberFormat("es-CO", { maximumFractionDigits: 2 });

/** Puntaje de anomalía como número legible ("3,25"); nunca como porcentaje. */
export function formatAnomalyScore(value: number): string {
    return Number.isFinite(value) ? ANOMALY_FORMATTER.format(value) : "—";
}
