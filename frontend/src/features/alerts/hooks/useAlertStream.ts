import { useEffect, useRef, useSyncExternalStore } from "react";
import { useQueryClient, type QueryClient } from "@tanstack/react-query";
import type { ActiveResponse, AlertId, PendingResponse } from "@/services/alertManagement";
import {
    createAlertStream,
    type AlertChangedEvent,
    type AlertNotificationEvent,
    type StreamStatus,
} from "@/services/realtime";
import { installAlertSoundUnlock, playAlertSound } from "../lib/alertSound";
import { showBrowserNotification } from "../lib/browserNotification";
import { applyToActive, applyToPending, isNewerEvaluation } from "../lib/streamCache";
import { alertKeys } from "./alertKeys";
import { useAlertDetailParam } from "./useAlertDetailParam";
import { applyNoticeChange, dismissNotification, receiveNotification } from "./useNewAlerts";

// Estado de la conexión a nivel de módulo: hay un solo stream para toda la app y varios lectores
// (el polling de las consultas y el indicador de conexión).
let status: StreamStatus = "idle";
const listeners = new Set<() => void>();

function setStatus(next: StreamStatus) {
    status = next;
    listeners.forEach((notify) => notify());
}

function subscribe(listener: () => void) {
    listeners.add(listener);
    return () => listeners.delete(listener);
}

/** Estado del stream de alertas. Con `live`, las consultas de alertas dejan de hacer polling. */
export function useAlertStreamStatus(): StreamStatus {
    return useSyncExternalStore(subscribe, () => status);
}

// Última foto aplicada de cada alerta: la caché de React Query no guarda `evaluated_at`.
const lastEvaluatedAt = new Map<AlertId, string>();

function applyAlertChanged(queryClient: QueryClient, event: AlertChangedEvent) {
    const levelIsNewer = isNewerEvaluation(event.evaluated_at, lastEvaluatedAt.get(event.alert_id));
    if (levelIsNewer) lastEvaluatedAt.set(event.alert_id, event.evaluated_at);

    let refetchActive = false;
    queryClient.setQueryData<ActiveResponse>(alertKeys.active(), (data) => {
        const update = applyToActive(data, event, levelIsNewer);
        refetchActive = update.refetch;
        return update.data;
    });
    if (refetchActive) void queryClient.invalidateQueries({ queryKey: alertKeys.active() });

    let refetchPending = false;
    queryClient.setQueryData<PendingResponse>(alertKeys.pending(), (data) => {
        const update = applyToPending(data, event, levelIsNewer);
        refetchPending = update.refetch;
        return update.data;
    });
    if (refetchPending) void queryClient.invalidateQueries({ queryKey: alertKeys.pending() });

    // El historial se marca como desactualizado sin recargarse (igual que tras una escritura propia) y el
    // detalle se vuelve a pedir solo si el modal de esa alerta está abierto.
    void queryClient.invalidateQueries({ queryKey: alertKeys.histories(), refetchType: "none" });
    void queryClient.invalidateQueries({ queryKey: alertKeys.management(event.alert_id) });
    // Vista combinada abierta en ese punto: el grupo se recalcula (una alerta nueva aparece como barra y pestaña).
    void queryClient.invalidateQueries({ queryKey: alertKeys.groups(event.measuring_point_id) });
}

/**
 * Abre el stream de alertas una sola vez para toda la app (se monta en RootLayout) y aplica sus eventos a la
 * caché. El stream sigue abierto con la pestaña oculta, para que los avisos lleguen aunque el operador esté
 * en otra pestaña.
 */
export function useAlertStream() {
    const queryClient = useQueryClient();
    const { open } = useAlertDetailParam();
    // El stream vive mientras dure la app; la notificación usa siempre el `open` más reciente.
    const openRef = useRef(open);
    useEffect(() => {
        openRef.current = open;
    }, [open]);

    useEffect(() => {
        const onNotification = (notice: AlertNotificationEvent) => {
            if (!receiveNotification(notice)) return;
            playAlertSound(notice);
            showBrowserNotification(notice, () => {
                dismissNotification(notice.alert_code);
                openRef.current(String(notice.alert_id));
            });
        };

        const stream = createAlertStream({
            handlers: {
                onChanged: (event) => {
                    applyAlertChanged(queryClient, event);
                    applyNoticeChange(event);
                },
                onNotification,
                onResync: () => {
                    void queryClient.invalidateQueries({ queryKey: alertKeys.active() });
                    void queryClient.invalidateQueries({ queryKey: alertKeys.pending() });
                },
                onStatusChange: setStatus,
            },
        });
        stream.connect();
        // El navegador bloquea el audio hasta que el usuario interactúa con la página.
        const removeSoundUnlock = installAlertSoundUnlock();
        return () => {
            stream.close();
            removeSoundUnlock();
        };
    }, [queryClient]);
}
