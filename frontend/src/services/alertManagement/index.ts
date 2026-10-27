import type { AlertManagementService } from "./alertManagement.service";
import { httpAlertManagementService } from "./alertManagement.http";

// Punto de composición: los hooks usan esta instancia; para pruebas o mocks se sustituye solo aquí.
export const alertManagementService: AlertManagementService = httpAlertManagementService;

export type { AlertManagementService } from "./alertManagement.service";
export { parseAlertId, toAlertId } from "./alertId";
export { LABELS_BY_CLASSIFICATION, MANAGEMENT_LIMITS, OTHER_LABEL } from "./alertManagement.types";
export type {
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
    TimelineMessage,
    TriggerVariable,
} from "./alertManagement.types";
