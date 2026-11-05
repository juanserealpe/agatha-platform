// Contrato del stream de alertas (NOTIFICATIONS_API.MD, GET /api/notifications/stream). Los campos siguen el
// JSON del backend tal cual (snake_case), igual que en services/alertManagement.
// Las fechas llegan en RFC 3339 UTC, p. ej. "2026-10-30T14:00:00Z".

import type {
    AlertEvent,
    AlertId,
    AlertLevel,
    AlertState,
    Classification,
    ManagementStatus,
} from "@/services/alertManagement";

/** Nombres de los eventos SSE. `ping` es el heartbeat: solo cuenta como actividad. */
export const STREAM_EVENTS = {
    changed: "alert.changed",
    notification: "alert.notification",
    resync: "resync",
    ping: "ping",
} as const;

/** Qué cambió. Sirve para logs y textos; para aplicar el estado basta la foto. */
export type AlertChange =
    | "OPENED"
    | "LEVEL_RAISED"
    | "LEVEL_LOWERED"
    | "REACTIVATED"
    | "FINISHED"
    | "NO_DATA_STARTED"
    | "NO_DATA_ENDED"
    | "CLASSIFIED"
    | "LABELED";

/** Aviso del catálogo de mensajes que se muestra al operador. */
export type NotificationCode =
    | "OPEN"
    | "NEW_AFTER_CLASSIFIED"
    | "UP_NARANJA_PERSIST"
    | "UP_NARANJA_FAST"
    | "UP_ROJO"
    | "REACTIVATED";

/**
 * `alert.changed`: foto del estado de la alerta justo después del cambio. Los eventos de prueba del backend
 * usan ids negativos, que nunca coinciden con una alerta real.
 */
export interface AlertChangedEvent {
    alert_id: AlertId;
    alert_code: string;
    /** El `id` del dispositivo (punto del mapa). */
    measuring_point_id: number;
    event_type: AlertEvent;
    change: AlertChange;
    state: AlertState;
    /** null si no hay nivel. */
    current_level: AlertLevel | null;
    max_level: AlertLevel;
    blinking: boolean;
    management_status: ManagementStatus;
    classification: Classification | null;
    started_at: string;
    /** Hora de procesamiento que produjo la foto: decide si es más nueva que la que hay en caché. */
    evaluated_at: string;
    occurred_at: string;
}

/** `alert.notification`: aviso para mostrar (banner, sonido, notificación del navegador). */
export interface AlertNotificationEvent {
    /** Clave para mostrar cada aviso una sola vez; una subida de nivel conserva el `alert_id`. */
    message_id: number;
    alert_id: AlertId;
    alert_code: string;
    measuring_point_id: number;
    event_type: AlertEvent;
    code: NotificationCode;
    level: AlertLevel;
    max_level: AlertLevel;
    classification: Classification | null;
    /** Ya redactado en español; se muestra como texto. */
    title: string;
    /** Ya redactado en español; se muestra como texto. */
    body: string;
    /** La hora llegó atrasada: se avisa igual, con la etiqueta "Atrasado". */
    late: boolean;
    started_at: string;
    evaluated_at: string;
    occurred_at: string;
}

/**
 * Estado de la conexión:
 * - `idle`: cerrado a propósito (o sin abrir).
 * - `connecting`: primer intento, todavía sin conectar.
 * - `live`: conectado; no hace falta el polling.
 * - `reconnecting`: se perdió la conexión y se está reintentando.
 * - `offline`: varios intentos seguidos fallaron; se sigue reintentando con más espera.
 */
export type StreamStatus = "idle" | "connecting" | "live" | "reconnecting" | "offline";

export interface AlertStreamHandlers {
    onChanged(event: AlertChangedEvent): void;
    onNotification(event: AlertNotificationEvent): void;
    /** Al conectar o reconectar, y con `resync`: hay que volver a pedir /active y /pending. */
    onResync(): void;
    onStatusChange(status: StreamStatus): void;
}
