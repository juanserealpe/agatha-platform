import type {
    ActiveResponse,
    AlertDetail,
    AlertId,
    AlertSearchFilters,
    AlertSearchResponse,
    ClassificationRequest,
    ExportedFile,
    LabelRequest,
    ManagementResult,
    ObservationRequest,
    PendingResponse,
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
    /** Las `limit` Sin clasificar más recientes (5 si se omite, máximo 50) y el total. */
    getPending(limit?: number, signal?: AbortSignal): Promise<PendingResponse>;
    /** Alertas activas agrupadas por punto de medición (mapa). */
    getActive(signal?: AbortSignal): Promise<ActiveResponse>;
    classify(id: AlertId, request: ClassificationRequest): Promise<ManagementResult>;
    label(id: AlertId, request: LabelRequest): Promise<ManagementResult>;
    addObservation(id: AlertId, request: ObservationRequest): Promise<ManagementResult>;
    /** Toda la búsqueda (sin paginar) como .xlsx. Rechaza con el mensaje del backend si supera 10.000 alertas. */
    exportXlsx(filters: AlertSearchFilters, signal?: AbortSignal): Promise<ExportedFile>;
}
