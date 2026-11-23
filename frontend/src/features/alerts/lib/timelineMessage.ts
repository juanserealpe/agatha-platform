import type { TimelineMessage } from "@/services/alertManagement";

/**
 * Variante visual de un mensaje de la línea de tiempo (spec multiindicador §7.1):
 * - `sensor`: evento del sensor, contexto común a fuga y movimiento. Son los `TEMP_*` (decisión A6).
 * - `management`: acción del operador (clasificación o etiqueta).
 * - `default`: notificaciones y actualizaciones del indicador.
 */
export type MessageVariant = "default" | "management" | "sensor";

// Por prefijo y no por lista cerrada: un código TEMP_* nuevo del catálogo queda marcado sin tocar el frontend.
const SENSOR_EVENT_PREFIX = "TEMP_";

export function messageVariant(message: Pick<TimelineMessage, "kind" | "code">): MessageVariant {
    if (message.code.startsWith(SENSOR_EVENT_PREFIX)) return "sensor";
    if (message.kind === "GESTION") return "management";
    return "default";
}
