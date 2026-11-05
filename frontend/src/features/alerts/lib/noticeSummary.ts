import type { AlertLevel } from "@/services/alertManagement";
import type { AlertNotificationEvent, NotificationCode } from "@/services/realtime";

// Marca que el catálogo del backend agrega al cuerpo cuando la línea base es de baja confianza
// (alert_management/domain/messages: LowConfidenceMark). Se conserva: avisa que el resultado es preliminar.
const LOW_CONFIDENCE_MARK = "Sensor en ajuste: resultado preliminar.";

/** Según el nivel: cuánto se aleja el ruido de lo normal. */
const LEVEL_SUMMARY: Record<AlertLevel, string> = {
    AMARILLO: "El índice de ruido está por encima de lo normal.",
    NARANJA: "El índice de ruido está muy por encima de lo normal.",
    ROJO: "El índice de ruido es demasiado alto.",
};

/** Según el aviso: qué cambió. null = se describe por el nivel. */
const CODE_SUMMARY: Record<NotificationCode, string | null> = {
    OPEN: null,
    NEW_AFTER_CLASSIFIED: null,
    UP_NARANJA_PERSIST: "El índice de ruido lleva varias horas por encima de lo normal.",
    UP_NARANJA_FAST: "El índice de ruido está subiendo más rápido de lo normal.",
    UP_ROJO: "El índice de ruido siguió subiendo y ya es demasiado alto.",
    REACTIVATED: "El índice de ruido volvió a subir después de haberse normalizado.",
};

/**
 * Cuerpo corto del banner y de la notificación del navegador. El backend manda un cuerpo con cifras (índice,
 * cortes, horas) pensado para el detalle; para un aviso de un vistazo se dice qué pasa en palabras simples.
 * Las cifras siguen en la línea de tiempo del detalle de la alerta.
 */
export function noticeSummary(notice: Pick<AlertNotificationEvent, "code" | "level" | "body">): string {
    const summary = CODE_SUMMARY[notice.code] ?? LEVEL_SUMMARY[notice.level];
    return notice.body.includes(LOW_CONFIDENCE_MARK) ? `${summary} ${LOW_CONFIDENCE_MARK}` : summary;
}
