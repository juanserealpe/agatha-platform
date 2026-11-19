import type { AlertManagementService } from "./alertManagement.service";
import { httpAlertManagementService } from "./alertManagement.http";

// Punto de composición: los hooks usan esta instancia; para pruebas o mocks se sustituye solo aquí.
export let alertManagementService: AlertManagementService = httpAlertManagementService;

/** Reemplaza la implementación (la demo usa una en memoria). Solo antes del primer render: no se cambia en caliente. */
export function installAlertManagementService(service: AlertManagementService) {
    alertManagementService = service;
}

export type { AlertManagementService } from "./alertManagement.service";
export { parseAlertId, toAlertId } from "./alertId";
export { LABELS_BY_CLASSIFICATION, MANAGEMENT_LIMITS, OTHER_LABEL } from "./alertManagement.types";
export type {
    ActiveDevice,
    ActiveResponse,
    AlertAction,
    AlertDetail,
    AlertEvent,
    AlertId,
    AlertItem,
    AlertLabel,
    AlertLevel,
    AlertSearchFilters,
    AlertSearchResponse,
    AlertState,
    Classification,
    ClassificationRequest,
    ConfirmedLabel,
    LeakSignature,
    DeviceRef,
    ExistingManagement,
    ExportedFile,
    FalseAlertLabel,
    LabelRequest,
    LevelOpenedBy,
    LevelSegment,
    ManagementOrigin,
    ManagementOutcome,
    ManagementResult,
    ManagementStatus,
    ManagementStep,
    MessageKind,
    MessageLevel,
    Observation,
    ObservationKind,
    ObservationRequest,
    ObservationStep,
    PendingResponse,
    TimelineMessage,
    TriggerVariable,
} from "./alertManagement.types";
