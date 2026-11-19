// Contrato del módulo Alert Management (ALERT_MANAGEMENT_API.MD). Los nombres de campo siguen el JSON del
// backend tal cual (snake_case): así los cuerpos que se envían coinciden exactamente con lo que acepta.
// Las fechas son RFC 3339 en hora de Colombia, p. ej. "2026-08-24T11:00:00-05:00".

/**
 * ID de la ALERTA (`items[].id`). Es un tipo propio para que el compilador impida usar por error el ID del
 * dispositivo (`device.id`) en las rutas de gestión: el backend lo tomaría como el ID de otra alerta.
 */
export type AlertId = number & { readonly __brand: "AlertId" };

export type AlertLevel = "AMARILLO" | "NARANJA" | "ROJO";
/** Los mensajes de la línea de tiempo también pueden venir en NORMAL (p. ej. el de fin). */
export type MessageLevel = AlertLevel | "NORMAL";

// Hoy cada alerta tiene un solo evento. Si el backend agrega uno combinado, basta sumarlo aquí: los
// catálogos de textos son Record<AlertEvent, …> y el compilador señala cada lugar que falte completar.
export type AlertEvent = "FUGA" | "MOVIMIENTO";

export type AlertState = "ACTIVA" | "FINALIZADA";

export type ManagementStatus = "SIN_CLASIFICAR" | "CLASIFICADA_SIN_ETIQUETA" | "CLASIFICADA_CON_ETIQUETA";

export type Classification = "CONFIRMADA" | "FALSA_ALERTA";

export type ConfirmedLabel = "FUGA_CONFIRMADA_CAMPO" | "MOVIMIENTO_TIERRA" | "MAQUINARIA_AJENA" | "OTRO";
export type FalseAlertLabel = "CLIMA_EXTREMO" | "TRANSITO_PESADO" | "MANTENIMIENTO_PROGRAMADO" | "OTRO";
export type AlertLabel = ConfirmedLabel | FalseAlertLabel;

export type AlertAction = "CONFIRM" | "FALSE_ALERT" | "ADD_LABEL" | "VIEW";

export type ManagementOrigin = "WEB" | "APP_MOVIL";

export type LevelOpenedBy = "OPEN" | "UP" | "DOWN" | "REACTIVATION";

export type MessageKind = "NOTIFICACION" | "ACTUALIZACION" | "RESUMEN" | "FIN" | "GESTION";

export type ObservationKind = "OBSERVACION" | "CONFLICTO_CLASIFICACION" | "CONFLICTO_ETIQUETA";

export type ObservationStep = "CLASIFICACION" | "ETIQUETA" | "LIBRE";

export type ManagementOutcome = "SAVED" | "ALREADY_RECEIVED" | "SAVED_AS_CONFLICT_OBSERVATION";

/** Etiqueta que exige texto libre (`other_text`). */
export const OTHER_LABEL = "OTRO" as const satisfies AlertLabel;

/** Etiquetas permitidas según la clasificación de la alerta (sección 5 del contrato). */
export const LABELS_BY_CLASSIFICATION = {
    CONFIRMADA: ["FUGA_CONFIRMADA_CAMPO", "MOVIMIENTO_TIERRA", "MAQUINARIA_AJENA", OTHER_LABEL],
    FALSA_ALERTA: ["CLIMA_EXTREMO", "TRANSITO_PESADO", "MANTENIMIENTO_PROGRAMADO", OTHER_LABEL],
} as const satisfies Record<Classification, readonly AlertLabel[]>;

/** Límites que valida el backend; el frontend los aplica antes de enviar. */
export const MANAGEMENT_LIMITS = {
    observationMaxLength: 500,
    otherTextMaxLength: 100,
    searchPageSize: 10,
    exportMaxAlerts: 10_000,
    /** `GET /pending` sin `limit` devuelve 5; el backend recorta a 50. */
    pendingMaxLimit: 50,
} as const;

export interface DeviceRef {
    /** `measuring_point_id`. Solo se usa en el filtro `device`, nunca en las rutas de gestión. */
    id: number;
    /** Nombre del dispositivo; "MP-<id>" si no está registrado. */
    pk: string;
}

/** Fila del historial. Las mutaciones devuelven la misma forma en `alert`. */
export interface AlertItem {
    id: AlertId;
    /** Código visible, p. ej. "ALR-000123". */
    code: string;
    device: DeviceRef;
    event: AlertEvent;
    state: AlertState;
    started_at: string;
    ended_at: string | null;
    in_progress: boolean;
    /** null si la alerta finalizó. */
    current_level: AlertLevel | null;
    max_level: AlertLevel;
    /** ROJO sin clasificar: se resalta la fila. */
    blinking: boolean;
    classification: Classification | null;
    management_status: ManagementStatus;
    label: AlertLabel | null;
    label_other_text: string | null;
    available_actions: AlertAction[];
    /**
     * Firma de fuga (planitud espectral) de la última hora calculada. null si la regla de planitud no está
     * activa en el punto, no hay datos o la alerta es anterior al campo. Solo informativo.
     */
    leak_signature?: LeakSignature | null;
}

export interface LeakSignature {
    /** % de registros de las últimas 24 h con planitud alta; null si la hora no lo trajo. */
    d_pl: number | null;
    /** Mínimo de la versión de umbrales del punto. */
    d_pl_min: number;
    /** La hora no se limitó a Amarillo y d_pl >= d_pl_min. */
    confirmed: boolean;
}

export interface AlertSearchResponse {
    total: number;
    page: number;
    page_size: number;
    items: AlertItem[];
}

/** `GET /pending`: las Sin clasificar más recientes y cuántas hay en total (panel). */
export interface PendingResponse {
    total: number;
    items: AlertItem[];
}

/** Punto del mapa con sus alertas activas. Solo aparecen los puntos que tienen al menos una. */
export interface ActiveDevice {
    measuring_point_id: number;
    /** Nombre del dispositivo; "MP-<id>" si no está registrado. */
    pk: string;
    /** La mayor criticidad entre sus alertas activas. */
    level: AlertLevel;
    /** Alguna de sus alertas está en ROJO sin clasificar. */
    blinking: boolean;
    alerts: AlertItem[];
}

/** `GET /active`, ordenado por `measuring_point_id`. */
export interface ActiveResponse {
    devices: ActiveDevice[];
}

/** Filtros del listado y la exportación. Dentro de un filtro se combinan con O; entre filtros, con Y. */
export interface AlertSearchFilters {
    /** `measuring_point_id`; null = todos. */
    device: number | null;
    /** Día de Colombia en formato aaaa-mm-dd: alertas que iniciaron desde ese día. */
    from: string | null;
    /** Día de Colombia en formato aaaa-mm-dd: alertas que iniciaron hasta ese día. */
    to: string | null;
    maxLevels: AlertLevel[];
    events: AlertEvent[];
    statuses: ManagementStatus[];
}

export interface TimelineMessage {
    id: number;
    at: string;
    kind: MessageKind;
    code: string;
    level: MessageLevel;
    indicator_value: number | null;
    /** Ya redactado para mostrar; se renderiza como texto. */
    title: string;
    /** Ya redactado para mostrar; se renderiza como texto. */
    body: string;
}

export interface LevelSegment {
    seq: number;
    opened_by: LevelOpenedBy;
    level: AlertLevel;
    started_at: string;
    ended_at: string | null;
    messages: TimelineMessage[];
}

export interface Observation {
    id: number;
    text: string;
    kind: ObservationKind;
    step: ObservationStep;
    origin: ManagementOrigin;
    device_identifier: string | null;
    recorded_at: string;
    received_at: string;
}

export interface ManagementStep<TValue> {
    value: TValue;
    /** Solo en la etiqueta OTRO; el backend lo omite en los demás casos. */
    other_text?: string | null;
    at: string | null;
    origin: ManagementOrigin;
    device_identifier: string | null;
    recorded_at: string | null;
}

/**
 * Variable que disparó la alerta frente a su rango normal.
 * Propuesta del frontend: el backend AÚN NO la envía; la UI funciona sin ella.
 */
export interface TriggerVariable {
    key: string;
    name: string;
    value: number;
    unit: string;
    normal_min: number;
    normal_max: number;
    deviation: number;
}

export interface AlertDetail extends AlertItem {
    opened_at: string;
    last_evaluated_at: string;
    reactivation_count: number;
    last_reactivated_at: string | null;
    /** Puntaje de anomalía al abrir. No es una probabilidad de fuga. */
    opening_indicator_value: number;
    /** Puntaje de anomalía en la última hora evaluada. No es una probabilidad de fuga. */
    last_indicator_value: number;
    hours_calculated: number;
    hours_no_data: number;
    low_confidence_at_open: boolean;
    /** Lo envía el backend aunque no está en el .md; no se muestra. */
    threshold_version_id_at_open?: number;
    classification_detail: ManagementStep<Classification> | null;
    label_detail: ManagementStep<AlertLabel> | null;
    observations: Observation[];
    levels: LevelSegment[];
    trigger_variables?: TriggerVariable[];
}

/** Campos comunes a toda escritura. Desde la web: `origin: "WEB"` y sin `device_identifier`. */
interface ManagementOriginFields {
    origin: ManagementOrigin;
    /** UUID por intento de guardado: reenviarlo no duplica el registro. */
    client_record_id?: string;
    device_identifier?: string;
    recorded_at?: string;
}

export interface ClassificationRequest extends ManagementOriginFields {
    classification: Classification;
    observation?: string;
}

export interface LabelRequest extends ManagementOriginFields {
    label: AlertLabel;
    /** Obligatorio con OTRO; con cualquier otra etiqueta NO se envía (el backend responde 400). */
    other_text?: string;
    observation?: string;
}

export interface ObservationRequest extends ManagementOriginFields {
    text: string;
}

/** Gestión que ya existía cuando otro usuario guardó primero (409). */
export interface ExistingManagement {
    classification?: Classification;
    classified_at?: string;
    label?: AlertLabel;
    label_other_text?: string;
    labeled_at?: string;
    origin: ManagementOrigin;
    device_identifier?: string;
}

export interface ManagementResult {
    outcome: ManagementOutcome;
    /** Fila vigente de la alerta, para reemplazarla en la tabla. */
    alert: AlertItem;
    /** Solo con SAVED_AS_CONFLICT_OBSERVATION. */
    existing?: ExistingManagement;
}

export interface ExportedFile {
    blob: Blob;
    /** null si la cabecera no llega o el navegador no la expone (CORS). */
    filename: string | null;
}
