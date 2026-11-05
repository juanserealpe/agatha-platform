import type { ActiveResponse, AlertItem, AlertLevel, ManagementStatus, PendingResponse } from "@/services/alertManagement";
import type { AlertChangedEvent } from "@/services/realtime";
import { LEVEL_ORDER } from "../config/alertManagementDisplay";

// Reducers puros que aplican un `alert.changed` (foto del estado de la alerta) a las cachés de /active y
// /pending. La foto no trae todo lo de la fila (nombre del dispositivo, etiqueta, acciones), así que solo
// se actualizan las alertas que ya están en caché; si aparece una que falta, se pide volver a consultar.

/** Resultado de aplicar un evento: la caché nueva (misma referencia si no cambió) y si hay que reconsultar. */
export interface CacheUpdate<T> {
    data: T;
    refetch: boolean;
}

const MANAGEMENT_RANK: Record<ManagementStatus, number> = {
    SIN_CLASIFICAR: 0,
    CLASIFICADA_SIN_ETIQUETA: 1,
    CLASIFICADA_CON_ETIQUETA: 2,
};

/** true si la foto `next` es más nueva que la última aplicada para esa alerta (o no se había aplicado ninguna). */
export function isNewerEvaluation(next: string, lastApplied: string | undefined): boolean {
    return lastApplied === undefined || Date.parse(next) > Date.parse(lastApplied);
}

/**
 * Fila con la foto aplicada. Nivel y estado solo si `levelIsNewer` (las fotos repetidas o viejas tras una
 * reconexión no hacen retroceder la alerta); la gestión solo si avanzó, porque nunca retrocede.
 */
export function mergeAlertItem(item: AlertItem, event: AlertChangedEvent, levelIsNewer: boolean): AlertItem {
    const level = levelIsNewer
        ? {
              state: event.state,
              in_progress: event.state === "ACTIVA",
              current_level: event.current_level,
              max_level: event.max_level,
              // La foto no trae la fecha de fin: se conserva la que haya; una reactivación la borra.
              ended_at: event.state === "ACTIVA" ? null : item.ended_at,
          }
        : {};
    const management =
        MANAGEMENT_RANK[event.management_status] > MANAGEMENT_RANK[item.management_status]
            ? { management_status: event.management_status, classification: event.classification }
            : {};

    const merged = { ...item, ...level, ...management };
    // Se recalcula para que salga coherente al combinar las dos reglas anteriores.
    merged.blinking =
        merged.state === "ACTIVA" && merged.current_level === "ROJO" && merged.management_status === "SIN_CLASIFICAR";
    return merged;
}

function maxLevel(levels: AlertLevel[]): AlertLevel {
    return levels.reduce((max, level) => (LEVEL_ORDER.indexOf(level) > LEVEL_ORDER.indexOf(max) ? level : max));
}

/** Puntos del mapa: actualiza la alerta, la quita si finalizó y recalcula el nivel y el titileo del punto. */
export function applyToActive(
    data: ActiveResponse | undefined,
    event: AlertChangedEvent,
    levelIsNewer: boolean
): CacheUpdate<ActiveResponse | undefined> {
    if (!data) return { data, refetch: false };

    const deviceIndex = data.devices.findIndex((device) => device.alerts.some((alert) => alert.id === event.alert_id));
    if (deviceIndex === -1) {
        // Una alerta activa que no está en caché (nueva o reactivada): hace falta su fila completa.
        return { data, refetch: event.state === "ACTIVA" && levelIsNewer };
    }

    const device = data.devices[deviceIndex];
    const alerts = device.alerts
        .map((alert) => (alert.id === event.alert_id ? mergeAlertItem(alert, event, levelIsNewer) : alert))
        .filter((alert) => alert.state === "ACTIVA");

    const devices = [...data.devices];
    if (alerts.length === 0) {
        devices.splice(deviceIndex, 1);
    } else {
        devices[deviceIndex] = {
            ...device,
            alerts,
            level: maxLevel(alerts.map((alert) => alert.current_level ?? alert.max_level)),
            blinking: alerts.some((alert) => alert.blinking),
        };
    }
    return { data: { devices }, refetch: false };
}

/**
 * Panel "Sin clasificar" (todas las SIN_CLASIFICAR, activas o finalizadas, de la más reciente a la más antigua).
 * Una alerta sale solo al clasificarse; una alerta nueva entra arriba, así que se reconsulta.
 */
export function applyToPending(
    data: PendingResponse | undefined,
    event: AlertChangedEvent,
    levelIsNewer: boolean
): CacheUpdate<PendingResponse | undefined> {
    if (!data) return { data, refetch: false };

    const index = data.items.findIndex((item) => item.id === event.alert_id);
    if (index === -1) {
        return { data, refetch: event.change === "OPENED" && event.management_status === "SIN_CLASIFICAR" };
    }

    const merged = mergeAlertItem(data.items[index], event, levelIsNewer);
    if (merged.management_status === "SIN_CLASIFICAR") {
        const items = [...data.items];
        items[index] = merged;
        return { data: { ...data, items }, refetch: false };
    }

    const items = data.items.filter((item) => item.id !== event.alert_id);
    const total = Math.max(0, data.total - 1);
    // Si había más pendientes que filas, se reconsulta para llenar el hueco.
    return { data: { total, items }, refetch: total > items.length };
}
