import type {
    AlertDetail,
    AlertId,
    AlertSearchFilters,
    AlertSearchResponse,
    ClassificationRequest,
    ExportedFile,
    LabelRequest,
    ManagementResult,
    ObservationRequest,
} from "./alertManagement.types";

/**
 * Contrato de la gestión de alertas. Los hooks dependen de esta interfaz, no de una implementación.
 * Todas las escrituras resuelven también el 409 por conflicto (otro usuario gestionó primero): no es un
 * error, el backend guarda el intento como observación y devuelve la fila vigente en `alert`.
 */
export interface AlertManagementService {
    /** Página de 10 alertas, de la más reciente a la más antigua por inicio. */
    search(filters: AlertSearchFilters, page: number, signal?: AbortSignal): Promise<AlertSearchResponse>;
    getDetail(id: AlertId, signal?: AbortSignal): Promise<AlertDetail>;
    classify(id: AlertId, request: ClassificationRequest): Promise<ManagementResult>;
    label(id: AlertId, request: LabelRequest): Promise<ManagementResult>;
    addObservation(id: AlertId, request: ObservationRequest): Promise<ManagementResult>;
    /** Toda la búsqueda (sin paginar) como .xlsx. Rechaza con el mensaje del backend si supera 10.000 alertas. */
    exportXlsx(filters: AlertSearchFilters, signal?: AbortSignal): Promise<ExportedFile>;
}
