import { useState } from "react";
import { useMutation, useQueryClient, type QueryClient } from "@tanstack/react-query";
import {
    alertManagementService,
    type AlertId,
    type AlertSearchResponse,
    type ManagementResult,
} from "@/services/alertManagement";
import { createClientRecordIdTracker } from "../lib/clientRecordId";
import { replaceHistoryItem } from "../lib/historyCache";
import {
    buildClassificationRequest,
    buildLabelRequest,
    buildObservationRequest,
    type ClassifyInput,
    type LabelInput,
    type ObservationInput,
} from "../lib/managementRequests";
import { alertKeys } from "./alertKeys";

/**
 * Refleja una escritura sin volver a pedir el listado: la fila se reemplaza en cada página en caché (se
 * conservan filtros y página actual) y el detalle se vuelve a consultar para mostrar la gestión guardada.
 */
function applyManagementResult(queryClient: QueryClient, result: ManagementResult) {
    queryClient.setQueriesData<AlertSearchResponse>({ queryKey: alertKeys.histories() }, (page) =>
        replaceHistoryItem(page, result.alert)
    );
    // Las búsquedas quedan marcadas como desactualizadas sin recargarse ahora: la fila clasificada sigue
    // visible aunque ya no cumpla el filtro de estado, y la próxima búsqueda o cambio de página trae lo vigente.
    void queryClient.invalidateQueries({ queryKey: alertKeys.histories(), refetchType: "none" });
    void queryClient.invalidateQueries({ queryKey: alertKeys.management(result.alert.id) });
}

function useManagementMutation<TInput>(
    alertId: AlertId,
    send: (id: AlertId, input: TInput, clientRecordId: string | undefined) => Promise<ManagementResult>
) {
    const queryClient = useQueryClient();
    // Un rastreador por formulario montado: reintentar el mismo guardado reutiliza su client_record_id.
    const [recordIds] = useState(createClientRecordIdTracker);

    return useMutation({
        mutationFn: (input: TInput) => send(alertId, input, recordIds.idFor(input)),
        onSuccess: (result) => {
            recordIds.reset();
            applyManagementResult(queryClient, result);
        },
    });
}

/** Confirmar o Falsa alerta (mismo endpoint). La clasificación es irreversible. */
export function useClassifyAlert(alertId: AlertId) {
    return useManagementMutation<ClassifyInput>(alertId, (id, input, recordId) =>
        alertManagementService.classify(id, buildClassificationRequest(input, recordId))
    );
}

export function useLabelAlert(alertId: AlertId) {
    return useManagementMutation<LabelInput>(alertId, (id, input, recordId) =>
        alertManagementService.label(id, buildLabelRequest(input, recordId))
    );
}

export function useAddObservation(alertId: AlertId) {
    return useManagementMutation<ObservationInput>(alertId, (id, input, recordId) =>
        alertManagementService.addObservation(id, buildObservationRequest(input, recordId))
    );
}
