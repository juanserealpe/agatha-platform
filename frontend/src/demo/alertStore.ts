import {
    LABELS_BY_CLASSIFICATION,
    MANAGEMENT_LIMITS,
    OTHER_LABEL,
    toAlertId,
    type ActiveDevice,
    type ActiveResponse,
    type AlertAction,
    type AlertDetail,
    type AlertId,
    type AlertItem,
    type AlertLabel,
    type AlertLevel,
    type AlertSearchFilters,
    type AlertSearchResponse,
    type Classification,
    type ClassificationRequest,
    type ExistingManagement,
    type LabelRequest,
    type LevelOpenedBy,
    type ManagementOrigin,
    type ManagementResult,
    type ManagementStatus,
    type ManagementStep,
    type MessageKind,
    type MessageLevel,
    type Observation,
    type ObservationKind,
    type ObservationRequest,
    type ObservationStep,
    type PendingResponse,
    type TimelineMessage,
} from "@/services/alertManagement";
import { ApiError } from "@/services/http/apiError";
import type {
    AlertChange,
    AlertChangedEvent,
    AlertNotificationEvent,
    NotificationCode,
} from "@/services/realtime/alertStream.types";
import {
    DEMO_THRESHOLDS,
    FIRST_DEMO_ALERT_ID,
    INDICATOR_BY_LEVEL,
    LEVEL_LEAD_HOURS,
    REACTIVATION_WINDOW_HOURS,
    SCENARIO_POINT_ID,
    findDemoPoint,
    type DemoPoint,
} from "./demoData";
import {
    addHours,
    colombiaDay,
    floorToHour,
    formatHour,
    formatValue,
    hoursBetween,
    toColombiaIso,
    toUtcIso,
} from "./demoFormat";

// Alert Management en memoria: guarda las alertas simuladas, responde las consultas con la forma exacta de la
// API (ALERT_MANAGEMENT_API.MD) y emite los eventos del stream (NOTIFICATIONS_API.MD) en cada cambio. Las reglas
// de gestión (conflictos, reenvíos, validaciones y textos) copian las del backend, para que la demo enseñe lo
// mismo que verá el operador con datos reales. Es puro: el reloj se inyecta y no usa APIs del navegador.

export type DemoStreamEvent =
    | { type: "changed"; data: AlertChangedEvent }
    | { type: "notification"; data: AlertNotificationEvent };

export type DemoStreamListener = (event: DemoStreamEvent) => void;

/** Tramo de un nivel en un punto, para que las series simuladas suban y bajen con las alertas. */
export interface LevelInterval {
    level: AlertLevel;
    from: Date;
    /** null si el nivel sigue abierto. */
    to: Date | null;
}

export interface DemoAlertStore {
    /** Abre una alerta de fuga. null si el punto ya tiene una activa (el backend no abre dos a la vez). */
    openAlert(level?: AlertLevel, measuringPointId?: number): AlertItem | null;
    /** Sube un nivel. null si la alerta no existe, terminó o ya está en ROJO. */
    raiseLevel(id: AlertId): AlertItem | null;
    /** El ruido vuelve a lo normal y la alerta termina. null si no existe o ya había terminado. */
    finishAlert(id: AlertId): AlertItem | null;
    /** La alerta activa del punto, si la hay. */
    activeAlertId(measuringPointId?: number): AlertId | null;
    levelIntervals(measuringPointId: number): LevelInterval[];
    /** Borra todo y vuelve a empezar en ALR-900001. */
    reset(): void;
    subscribe(listener: DemoStreamListener): () => void;

    search(filters: AlertSearchFilters, page: number): AlertSearchResponse;
    getDetail(id: AlertId): AlertDetail;
    getPending(limit?: number): PendingResponse;
    getActive(): ActiveResponse;
    classify(id: AlertId, request: ClassificationRequest): ManagementResult;
    label(id: AlertId, request: LabelRequest): ManagementResult;
    addObservation(id: AlertId, request: ObservationRequest): ManagementResult;
}

const LEVEL_ORDER: readonly AlertLevel[] = ["AMARILLO", "NARANJA", "ROJO"];

const LEVEL_NAME: Record<AlertLevel, string> = { AMARILLO: "Amarillo", NARANJA: "Naranja", ROJO: "Rojo" };

const CLASSIFICATION_NAME: Record<Classification, string> = {
    CONFIRMADA: "Confirmada",
    FALSA_ALERTA: "Falsa alerta",
};

// Textos de campo del backend (entities.labelNames): con OTRO se usa el texto libre.
const LABEL_NAME: Record<AlertLabel, string> = {
    FUGA_CONFIRMADA_CAMPO: "Fuga confirmada en campo",
    MOVIMIENTO_TIERRA: "Movimiento de tierra",
    MAQUINARIA_AJENA: "Intervención de maquinaria ajena",
    CLIMA_EXTREMO: "Condiciones climáticas extremas",
    TRANSITO_PESADO: "Tránsito de vehículo pesado",
    MANTENIMIENTO_PROGRAMADO: "Mantenimiento o intervención programada",
    OTRO: "Otro",
};

const ACTIONS_BY_STATUS: Record<ManagementStatus, AlertAction[]> = {
    SIN_CLASIFICAR: ["CONFIRM", "FALSE_ALERT", "VIEW"],
    CLASIFICADA_SIN_ETIQUETA: ["ADD_LABEL", "VIEW"],
    CLASIFICADA_CON_ETIQUETA: ["VIEW"],
};

const DEFAULT_PENDING_LIMIT = 5;

// Mensajes de error del backend (alert_management/domain/errors.go).
const ERRORS = {
    notFound: { status: 404, code: "NOT_FOUND", message: "la alerta no existe" },
    invalidOrigin: {
        status: 400,
        code: "INVALID_INPUT",
        message: "origin debe ser WEB o APP_MOVIL; desde la app, device_identifier es obligatorio",
    },
    invalidClassification: {
        status: 400,
        code: "INVALID_INPUT",
        message: "classification debe ser CONFIRMADA o FALSA_ALERTA",
    },
    invalidLabel: {
        status: 400,
        code: "INVALID_INPUT",
        message: "la etiqueta no pertenece a la lista de la clasificación de la alerta",
    },
    otherTextRequired: {
        status: 400,
        code: "INVALID_INPUT",
        message: "con la etiqueta OTRO, other_text es obligatorio (1 a 100 caracteres); con otra etiqueta no se envía",
    },
    invalidObservation: {
        status: 400,
        code: "INVALID_INPUT",
        message: "la observación debe tener entre 1 y 500 caracteres",
    },
    notClassified: {
        status: 409,
        code: "CONFLICT",
        message: "la alerta todavía no está clasificada: primero clasificar, después etiquetar",
    },
} as const;

interface StoredSegment {
    seq: number;
    openedBy: LevelOpenedBy;
    level: AlertLevel;
    startedAt: Date;
    endedAt: Date | null;
    messages: TimelineMessage[];
}

interface StoredAlert {
    id: AlertId;
    code: string;
    point: DemoPoint;
    currentLevel: AlertLevel | null;
    maxLevel: AlertLevel;
    startedAt: Date;
    /** Hora de procesamiento que abrió la alerta; el inicio (`startedAt`) es anterior. */
    openedAt: Date;
    endedAt: Date | null;
    lastEvaluatedAt: Date;
    openingIndicator: number;
    lastIndicator: number;
    classification: ManagementStep<Classification> | null;
    label: ManagementStep<AlertLabel> | null;
    observations: Observation[];
    segments: StoredSegment[];
}

interface MessageInput {
    kind: MessageKind;
    code: string;
    level: MessageLevel;
    indicator: number | null;
    title: string;
    body: string;
}

export interface DemoAlertStoreOptions {
    now?: () => Date;
}

export function alertCode(id: number): string {
    return `ALR-${String(id).padStart(6, "0")}`;
}

function apiError(error: (typeof ERRORS)[keyof typeof ERRORS], body: unknown = null): ApiError {
    return new ApiError({ ...error, body });
}

function originText(origin: ManagementOrigin, deviceIdentifier: string | null): string {
    if (origin === "WEB") return "Web";
    return deviceIdentifier ? `App móvil (${deviceIdentifier})` : "App móvil";
}

function labelText(label: AlertLabel, otherText: string): string {
    return label === OTHER_LABEL ? `Otro: ${otherText}` : LABEL_NAME[label];
}

function managementStatus(alert: StoredAlert): ManagementStatus {
    if (alert.classification === null) return "SIN_CLASIFICAR";
    return alert.label === null ? "CLASIFICADA_SIN_ETIQUETA" : "CLASIFICADA_CON_ETIQUETA";
}

function isActive(alert: StoredAlert): boolean {
    return alert.endedAt === null;
}

function isBlinking(alert: StoredAlert): boolean {
    return isActive(alert) && alert.currentLevel === "ROJO" && alert.classification === null;
}

function lastSegment(alert: StoredAlert): StoredSegment {
    return alert.segments[alert.segments.length - 1];
}

function validateOrigin(request: { origin: ManagementOrigin; device_identifier?: string }) {
    const fromApp = request.origin === "APP_MOVIL";
    if ((request.origin !== "WEB" && !fromApp) || (fromApp && !request.device_identifier?.trim())) {
        throw apiError(ERRORS.invalidOrigin);
    }
}

/** Observación opcional ya recortada; "" si no vino. */
function optionalObservation(text: string | undefined): string {
    const trimmed = text?.trim() ?? "";
    if ([...trimmed].length > MANAGEMENT_LIMITS.observationMaxLength) throw apiError(ERRORS.invalidObservation);
    return trimmed;
}

function toItem(alert: StoredAlert): AlertItem {
    const status = managementStatus(alert);
    return {
        id: alert.id,
        code: alert.code,
        device: { id: alert.point.id, pk: alert.point.name },
        event: "FUGA",
        state: isActive(alert) ? "ACTIVA" : "FINALIZADA",
        started_at: toColombiaIso(alert.startedAt),
        ended_at: alert.endedAt ? toColombiaIso(alert.endedAt) : null,
        in_progress: isActive(alert),
        current_level: alert.currentLevel,
        max_level: alert.maxLevel,
        blinking: isBlinking(alert),
        classification: alert.classification?.value ?? null,
        management_status: status,
        label: alert.label?.value ?? null,
        label_other_text: alert.label?.other_text ?? null,
        available_actions: [...ACTIONS_BY_STATUS[status]],
    };
}

function toDetail(alert: StoredAlert): AlertDetail {
    return {
        ...toItem(alert),
        opened_at: toColombiaIso(alert.openedAt),
        last_evaluated_at: toColombiaIso(alert.lastEvaluatedAt),
        reactivation_count: 0,
        last_reactivated_at: null,
        opening_indicator_value: alert.openingIndicator,
        last_indicator_value: alert.lastIndicator,
        hours_calculated: hoursBetween(alert.startedAt, alert.endedAt ?? alert.lastEvaluatedAt),
        hours_no_data: 0,
        low_confidence_at_open: false,
        classification_detail: alert.classification ? { ...alert.classification } : null,
        label_detail: alert.label ? { ...alert.label } : null,
        observations: alert.observations.map((observation) => ({ ...observation })),
        levels: alert.segments.map((segment) => ({
            seq: segment.seq,
            opened_by: segment.openedBy,
            level: segment.level,
            started_at: toColombiaIso(segment.startedAt),
            ended_at: segment.endedAt ? toColombiaIso(segment.endedAt) : null,
            messages: segment.messages.map((message) => ({ ...message })),
        })),
    };
}

function existingManagement(alert: StoredAlert, origin: ManagementStep<unknown>): ExistingManagement {
    return {
        ...(alert.classification && {
            classification: alert.classification.value,
            classified_at: alert.classification.at ?? undefined,
        }),
        ...(alert.label && {
            label: alert.label.value,
            labeled_at: alert.label.at ?? undefined,
            ...(alert.label.other_text && { label_other_text: alert.label.other_text }),
        }),
        origin: origin.origin,
        ...(origin.device_identifier && { device_identifier: origin.device_identifier }),
    };
}

function matchesFilters(alert: StoredAlert, filters: AlertSearchFilters): boolean {
    const day = colombiaDay(alert.startedAt);
    return (
        (filters.device === null || alert.point.id === filters.device) &&
        (filters.from === null || day >= filters.from) &&
        (filters.to === null || day <= filters.to) &&
        (filters.maxLevels.length === 0 || filters.maxLevels.includes(alert.maxLevel)) &&
        (filters.events.length === 0 || filters.events.includes("FUGA")) &&
        (filters.statuses.length === 0 || filters.statuses.includes(managementStatus(alert)))
    );
}

/** De la más reciente a la más antigua por inicio, como el listado del backend. */
function byNewest(a: StoredAlert, b: StoredAlert): number {
    return b.startedAt.getTime() - a.startedAt.getTime() || b.id - a.id;
}

export function createDemoAlertStore({ now = () => new Date() }: DemoAlertStoreOptions = {}): DemoAlertStore {
    let alerts = new Map<number, StoredAlert>();
    let nextAlertId = FIRST_DEMO_ALERT_ID;
    let nextMessageId = 1;
    let nextObservationId = 1;
    let lastEvaluation = 0;
    let receivedRecords = new Set<string>();
    const listeners = new Set<DemoStreamListener>();

    function emit(event: DemoStreamEvent) {
        for (const listener of listeners) listener(event);
    }

    // El cliente descarta una foto si su `evaluated_at` no es mayor que la que tiene: dos pasos en el mismo
    // segundo deben quedar en segundos distintos.
    function nextEvaluation(): Date {
        const second = Math.floor(now().getTime() / 1000) * 1000;
        lastEvaluation = Math.max(second, lastEvaluation + 1000);
        return new Date(lastEvaluation);
    }

    function find(id: AlertId): StoredAlert {
        const alert = alerts.get(id);
        if (!alert) throw apiError(ERRORS.notFound);
        return alert;
    }

    function pointAlerts(measuringPointId: number): StoredAlert[] {
        return [...alerts.values()].filter((alert) => alert.point.id === measuringPointId);
    }

    /** Último límite de tiempo ya ocupado en el punto: un nivel nuevo nunca empieza antes. */
    function latestBoundary(measuringPointId: number): number {
        let latest = 0;
        for (const alert of pointAlerts(measuringPointId)) {
            for (const segment of alert.segments) {
                latest = Math.max(latest, segment.endedAt?.getTime() ?? segment.startedAt.getTime());
            }
        }
        return latest;
    }

    function levelStart(level: AlertLevel, evaluatedAt: Date, measuringPointId: number): Date {
        const backdated = addHours(floorToHour(evaluatedAt), -LEVEL_LEAD_HOURS[level]).getTime();
        return new Date(Math.max(backdated, latestBoundary(measuringPointId)));
    }

    function addMessage(alert: StoredAlert, input: MessageInput, at: Date): number {
        const id = nextMessageId++;
        lastSegment(alert).messages.push({
            id,
            at: toColombiaIso(at),
            kind: input.kind,
            code: input.code,
            level: input.level,
            indicator_value: input.indicator,
            title: input.title,
            body: input.body,
        });
        return id;
    }

    function addObservation(
        alert: StoredAlert,
        text: string,
        kind: ObservationKind,
        step: ObservationStep,
        request: { origin: ManagementOrigin; device_identifier?: string; recorded_at?: string },
        receivedAt: Date
    ) {
        alert.observations.push({
            id: nextObservationId++,
            text,
            kind,
            step,
            origin: request.origin,
            device_identifier: request.device_identifier ?? null,
            recorded_at: toColombiaIso(request.recorded_at ? new Date(request.recorded_at) : receivedAt),
            received_at: toColombiaIso(receivedAt),
        });
    }

    function emitChanged(alert: StoredAlert, change: AlertChange, occurredAt: Date) {
        const item = toItem(alert);
        emit({
            type: "changed",
            data: {
                alert_id: alert.id,
                alert_code: alert.code,
                measuring_point_id: alert.point.id,
                event_type: "FUGA",
                change,
                state: item.state,
                current_level: alert.currentLevel,
                max_level: alert.maxLevel,
                blinking: item.blinking,
                management_status: item.management_status,
                classification: item.classification,
                started_at: toUtcIso(alert.startedAt),
                evaluated_at: toUtcIso(alert.lastEvaluatedAt),
                occurred_at: toUtcIso(occurredAt),
            },
        });
    }

    function emitNotification(alert: StoredAlert, messageId: number, code: NotificationCode, input: MessageInput) {
        emit({
            type: "notification",
            data: {
                message_id: messageId,
                alert_id: alert.id,
                alert_code: alert.code,
                measuring_point_id: alert.point.id,
                event_type: "FUGA",
                code,
                level: alert.currentLevel ?? alert.maxLevel,
                max_level: alert.maxLevel,
                classification: alert.classification?.value ?? null,
                title: input.title,
                body: input.body,
                late: false,
                started_at: toUtcIso(alert.startedAt),
                evaluated_at: toUtcIso(alert.lastEvaluatedAt),
                occurred_at: toUtcIso(alert.lastEvaluatedAt),
            },
        });
    }

    /** Idempotencia por `client_record_id`: el mismo intento otra vez no tiene efecto. */
    function alreadyReceived(alert: StoredAlert, clientRecordId: string | undefined): ManagementResult | null {
        if (!clientRecordId) return null;
        if (receivedRecords.has(clientRecordId)) return { outcome: "ALREADY_RECEIVED", alert: toItem(alert) };
        receivedRecords.add(clientRecordId);
        return null;
    }

    function managementStep<TValue>(
        value: TValue,
        request: { origin: ManagementOrigin; device_identifier?: string; recorded_at?: string },
        at: Date
    ): ManagementStep<TValue> {
        return {
            value,
            at: toColombiaIso(at),
            origin: request.origin,
            device_identifier: request.device_identifier ?? null,
            recorded_at: toColombiaIso(request.recorded_at ? new Date(request.recorded_at) : at),
        };
    }

    return {
        openAlert(level = "AMARILLO", measuringPointId = SCENARIO_POINT_ID) {
            const point = findDemoPoint(measuringPointId);
            if (!point || pointAlerts(measuringPointId).some(isActive)) return null;

            const evaluatedAt = nextEvaluation();
            const startedAt = levelStart(level, evaluatedAt, measuringPointId);
            const id = toAlertId(nextAlertId++);
            const indicator = INDICATOR_BY_LEVEL[level];
            const alert: StoredAlert = {
                id,
                code: alertCode(id),
                point,
                currentLevel: level,
                maxLevel: level,
                startedAt,
                openedAt: evaluatedAt,
                endedAt: null,
                lastEvaluatedAt: evaluatedAt,
                openingIndicator: indicator,
                lastIndicator: indicator,
                classification: null,
                label: null,
                observations: [],
                segments: [{ seq: 1, openedBy: "OPEN", level, startedAt, endedAt: null, messages: [] }],
            };
            alerts.set(id, alert);

            const normal = formatValue(DEMO_THRESHOLDS.amarillo);
            const cut = level === "NARANJA" ? DEMO_THRESHOLDS.naranja : DEMO_THRESHOLDS.rojo;
            const message: MessageInput = {
                kind: "NOTIFICACION",
                code: "OPEN",
                level,
                indicator,
                title: `Alerta nueva en ${point.name}: nivel ${LEVEL_NAME[level]}`,
                body:
                    level === "AMARILLO"
                        ? `Índice de ruido en ${formatValue(indicator)} desde las ${formatHour(startedAt)}; lo normal es menos de ${normal}.`
                        : `Índice de ruido en ${formatValue(indicator)} desde las ${formatHour(startedAt)}; ${LEVEL_NAME[level]} empieza en ${formatValue(cut)} y lo normal es menos de ${normal}.`,
            };
            const messageId = addMessage(alert, message, evaluatedAt);
            emitChanged(alert, "OPENED", evaluatedAt);
            emitNotification(alert, messageId, "OPEN", message);
            return toItem(alert);
        },

        raiseLevel(id) {
            const alert = alerts.get(id);
            const current = alert?.currentLevel;
            if (!alert || !current || !isActive(alert) || current === "ROJO") return null;

            const level = LEVEL_ORDER[LEVEL_ORDER.indexOf(current) + 1];
            const evaluatedAt = nextEvaluation();
            const startedAt = levelStart(level, evaluatedAt, alert.point.id);
            const previous = lastSegment(alert);
            previous.endedAt = startedAt;
            alert.segments.push({
                seq: previous.seq + 1,
                openedBy: "UP",
                level,
                startedAt,
                endedAt: null,
                messages: [],
            });
            const before = alert.lastIndicator;
            alert.currentLevel = level;
            alert.maxLevel = level;
            alert.lastIndicator = INDICATOR_BY_LEVEL[level];
            alert.lastEvaluatedAt = evaluatedAt;

            const code: NotificationCode = level === "ROJO" ? "UP_ROJO" : "UP_NARANJA_FAST";
            const cut = level === "ROJO" ? DEMO_THRESHOLDS.rojo : DEMO_THRESHOLDS.naranja;
            const message: MessageInput = {
                kind: "NOTIFICACION",
                code,
                level,
                indicator: alert.lastIndicator,
                title: `La alerta ${alert.code} subió a ${LEVEL_NAME[level]}`,
                body: `El índice de ruido pasó de ${formatValue(before)} a ${formatValue(alert.lastIndicator)}; ${LEVEL_NAME[level]} empieza en ${formatValue(cut)}.`,
            };
            const messageId = addMessage(alert, message, evaluatedAt);
            emitChanged(alert, "LEVEL_RAISED", evaluatedAt);
            emitNotification(alert, messageId, code, message);
            return toItem(alert);
        },

        finishAlert(id) {
            const alert = alerts.get(id);
            if (!alert || !isActive(alert)) return null;

            const evaluatedAt = nextEvaluation();
            const peak = INDICATOR_BY_LEVEL[alert.maxLevel];
            lastSegment(alert).endedAt = evaluatedAt;
            alert.endedAt = evaluatedAt;
            alert.currentLevel = null;
            alert.lastIndicator = INDICATOR_BY_LEVEL.NORMAL;
            alert.lastEvaluatedAt = evaluatedAt;

            // El mensaje de fin no es un aviso: no va por alert.notification.
            addMessage(
                alert,
                {
                    kind: "FIN",
                    code: "END_NORMAL",
                    level: "NORMAL",
                    indicator: alert.lastIndicator,
                    title: `La alerta ${alert.code} terminó`,
                    body: `El índice de ruido volvió a ${formatValue(alert.lastIndicator)} (lo normal, menos de ${formatValue(DEMO_THRESHOLDS.amarillo)}). Duró ${hoursBetween(alert.startedAt, evaluatedAt)} h; nivel máximo ${LEVEL_NAME[alert.maxLevel]}, con índice de hasta ${formatValue(peak)}. Si vuelve en las próximas ${REACTIVATION_WINDOW_HOURS} h y sigue sin clasificar, se reactivará esta misma alerta.`,
                },
                evaluatedAt
            );
            emitChanged(alert, "FINISHED", evaluatedAt);
            return toItem(alert);
        },

        activeAlertId(measuringPointId = SCENARIO_POINT_ID) {
            return pointAlerts(measuringPointId).find(isActive)?.id ?? null;
        },

        levelIntervals(measuringPointId) {
            return pointAlerts(measuringPointId)
                .flatMap((alert) => alert.segments)
                .map((segment) => ({ level: segment.level, from: segment.startedAt, to: segment.endedAt }))
                .sort((a, b) => a.from.getTime() - b.from.getTime());
        },

        reset() {
            alerts = new Map();
            nextAlertId = FIRST_DEMO_ALERT_ID;
            nextMessageId = 1;
            nextObservationId = 1;
            receivedRecords = new Set();
        },

        subscribe(listener) {
            listeners.add(listener);
            return () => {
                listeners.delete(listener);
            };
        },

        search(filters, page) {
            const pageSize = MANAGEMENT_LIMITS.searchPageSize;
            const matching = [...alerts.values()].filter((alert) => matchesFilters(alert, filters)).sort(byNewest);
            return {
                total: matching.length,
                page,
                page_size: pageSize,
                items: matching.slice((page - 1) * pageSize, page * pageSize).map(toItem),
            };
        },

        getDetail(id) {
            return toDetail(find(id));
        },

        getPending(limit = DEFAULT_PENDING_LIMIT) {
            const size = Math.min(Math.max(limit, 1), MANAGEMENT_LIMITS.pendingMaxLimit);
            const pending = [...alerts.values()].filter((alert) => alert.classification === null).sort(byNewest);
            return { total: pending.length, items: pending.slice(0, size).map(toItem) };
        },

        getActive() {
            const byPoint = new Map<number, StoredAlert[]>();
            for (const alert of [...alerts.values()].filter(isActive).sort(byNewest)) {
                byPoint.set(alert.point.id, [...(byPoint.get(alert.point.id) ?? []), alert]);
            }
            const devices: ActiveDevice[] = [...byPoint.entries()]
                .sort(([a], [b]) => a - b)
                .map(([pointId, pointList]) => ({
                    measuring_point_id: pointId,
                    pk: pointList[0].point.name,
                    level: pointList
                        .map((alert) => alert.currentLevel ?? alert.maxLevel)
                        .reduce((max, level) => (LEVEL_ORDER.indexOf(level) > LEVEL_ORDER.indexOf(max) ? level : max)),
                    blinking: pointList.some(isBlinking),
                    alerts: pointList.map(toItem),
                }));
            return { devices };
        },

        classify(id, request) {
            const alert = find(id);
            if (request.classification !== "CONFIRMADA" && request.classification !== "FALSA_ALERTA") {
                throw apiError(ERRORS.invalidClassification);
            }
            validateOrigin(request);
            const observation = optionalObservation(request.observation);
            const received = alreadyReceived(alert, request.client_record_id);
            if (received) return received;

            const at = now();
            if (alert.classification !== null) {
                const text = `Clasificación en conflicto: ${CLASSIFICATION_NAME[request.classification]} desde ${originText(request.origin, request.device_identifier ?? null)}`;
                const full = (observation ? `${text}. ${observation}` : text).slice(0, MANAGEMENT_LIMITS.observationMaxLength);
                addObservation(alert, full, "CONFLICTO_CLASIFICACION", "CLASIFICACION", request, at);
                return {
                    outcome: "SAVED_AS_CONFLICT_OBSERVATION",
                    alert: toItem(alert),
                    existing: existingManagement(alert, alert.classification),
                };
            }

            alert.classification = managementStep(request.classification, request, at);
            if (observation) addObservation(alert, observation, "OBSERVACION", "CLASIFICACION", request, at);
            addMessage(
                alert,
                {
                    kind: "GESTION",
                    code: "CLASSIFIED",
                    level: alert.currentLevel ?? "NORMAL",
                    indicator: null,
                    title: `Clasificada como ${CLASSIFICATION_NAME[request.classification]}`,
                    body: `Registrada desde ${originText(request.origin, request.device_identifier ?? null)}.`,
                },
                at
            );
            emitChanged(alert, "CLASSIFIED", at);
            return { outcome: "SAVED", alert: toItem(alert) };
        },

        label(id, request) {
            const alert = find(id);
            validateOrigin(request);
            const otherText = request.other_text?.trim() ?? "";
            if (request.label === OTHER_LABEL) {
                const length = [...otherText].length;
                if (length < 1 || length > MANAGEMENT_LIMITS.otherTextMaxLength) throw apiError(ERRORS.otherTextRequired);
            } else if (otherText !== "") {
                throw apiError(ERRORS.otherTextRequired);
            }
            const observation = optionalObservation(request.observation);
            const received = alreadyReceived(alert, request.client_record_id);
            if (received) return received;

            if (alert.classification === null) throw apiError(ERRORS.notClassified);
            const allowed: readonly AlertLabel[] = LABELS_BY_CLASSIFICATION[alert.classification.value];
            if (alert.label === null && !allowed.includes(request.label)) throw apiError(ERRORS.invalidLabel);

            const at = now();
            const origin = originText(request.origin, request.device_identifier ?? null);
            if (alert.label !== null) {
                const text = `Etiqueta en conflicto: ${labelText(request.label, otherText)} desde ${origin}`;
                const full = (observation ? `${text}. ${observation}` : text).slice(0, MANAGEMENT_LIMITS.observationMaxLength);
                addObservation(alert, full, "CONFLICTO_ETIQUETA", "ETIQUETA", request, at);
                return {
                    outcome: "SAVED_AS_CONFLICT_OBSERVATION",
                    alert: toItem(alert),
                    existing: existingManagement(alert, alert.label),
                };
            }

            alert.label = {
                ...managementStep(request.label, request, at),
                ...(request.label === OTHER_LABEL && { other_text: otherText }),
            };
            if (observation) addObservation(alert, observation, "OBSERVACION", "ETIQUETA", request, at);
            addMessage(
                alert,
                {
                    kind: "GESTION",
                    code: "LABELED",
                    level: alert.currentLevel ?? "NORMAL",
                    indicator: null,
                    title: `Etiqueta: ${labelText(request.label, otherText)}`,
                    body: `Registrada desde ${origin}.`,
                },
                at
            );
            emitChanged(alert, "LABELED", at);
            return { outcome: "SAVED", alert: toItem(alert) };
        },

        addObservation(id, request) {
            const alert = find(id);
            validateOrigin(request);
            const text = request.text.trim();
            const length = [...text].length;
            if (length < 1 || length > MANAGEMENT_LIMITS.observationMaxLength) throw apiError(ERRORS.invalidObservation);
            const received = alreadyReceived(alert, request.client_record_id);
            if (received) return received;

            addObservation(alert, text, "OBSERVACION", "LIBRE", request, now());
            return { outcome: "SAVED", alert: toItem(alert) };
        },
    };
}
