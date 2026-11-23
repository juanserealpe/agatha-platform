import { apiPath, httpClient, isApiError, type QueryParams } from "@/services/http";
import type { AlertManagementService } from "./alertManagement.service";
import type { AlertSearchFilters, ManagementResult } from "./alertManagement.types";

const ALERTS_PATH = ["api", "alerts"] as const;

const CONFLICT_STATUS = 409;

/** Parámetros del listado y la exportación. Los valores múltiples se repiten (`max_level=NARANJA&max_level=ROJO`). */
export function toSearchQuery(filters: AlertSearchFilters): QueryParams {
    return {
        device: filters.device,
        from: filters.from,
        to: filters.to,
        max_level: filters.maxLevels,
        event: filters.events,
        status: filters.statuses,
    };
}

function isConflictResult(body: unknown): body is ManagementResult {
    return (
        typeof body === "object" &&
        body !== null &&
        "outcome" in body &&
        body.outcome === "SAVED_AS_CONFLICT_OBSERVATION" &&
        "alert" in body
    );
}

async function postManagement(path: string, request: object): Promise<ManagementResult> {
    try {
        const { data } = await httpClient.post<ManagementResult>(path, request);
        return data;
    } catch (error) {
        // 409 con `outcome`: otro usuario gestionó primero y prevalece lo suyo. Es un resultado válido.
        // Un 409 sin `outcome` (p. ej. etiquetar sin clasificar) sí es un error y conserva su `message`.
        if (isApiError(error) && error.status === CONFLICT_STATUS && isConflictResult(error.body)) {
            return error.body;
        }
        throw error;
    }
}

export const httpAlertManagementService: AlertManagementService = {
    search: (filters, page, signal) =>
        httpClient.get(apiPath(...ALERTS_PATH), {
            query: { ...toSearchQuery(filters), page },
            signal,
            errorMessage: "No fue posible consultar las alertas. Intente nuevamente.",
        }),

    getDetail: (id, signal) =>
        httpClient.get(apiPath(...ALERTS_PATH, id), {
            signal,
            errorMessage: "No fue posible cargar el detalle de la alerta. Intente nuevamente.",
        }),

    getPending: (limit, signal) =>
        httpClient.get(apiPath(...ALERTS_PATH, "pending"), {
            query: { limit },
            signal,
            errorMessage: "No fue posible consultar las alertas sin clasificar. Intente nuevamente.",
        }),

    getActive: (signal) =>
        httpClient.get(apiPath(...ALERTS_PATH, "active"), {
            signal,
            errorMessage: "No fue posible consultar las alertas activas. Intente nuevamente.",
        }),

    // Única ruta del módulo donde el id es del DISPOSITIVO (por el prefijo /devices/).
    getDeviceAlerts: (deviceId, days, signal) =>
        httpClient.get(apiPath(...ALERTS_PATH, "devices", deviceId, "alerts"), {
            query: { from: days.from, to: days.to },
            signal,
            errorMessage: "No fue posible consultar las alertas del punto. Intente nuevamente.",
        }),

    classify: (id, request) => postManagement(apiPath(...ALERTS_PATH, id, "classification"), request),

    label: (id, request) => postManagement(apiPath(...ALERTS_PATH, id, "label"), request),

    addObservation: (id, request) => postManagement(apiPath(...ALERTS_PATH, id, "observations"), request),

    exportXlsx: (filters, signal) =>
        httpClient.getBlob(apiPath(...ALERTS_PATH, "export.xlsx"), {
            query: toSearchQuery(filters),
            signal,
            errorMessage: "No fue posible exportar las alertas. Intente nuevamente.",
        }),
};
