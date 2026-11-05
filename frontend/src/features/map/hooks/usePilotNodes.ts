import { useMemo } from "react";
import { mostUrgentAlert, useActiveAlerts } from "@/features/alerts";
import { extractPk, getDeviceStatus, useDevices } from "@/features/sensors";
import type { ActiveDevice, AlertItem } from "@/services/alertManagement";
import type { Device } from "@/services/dashboard/dashboard.types";
import { useNow } from "@/shared/hooks/useNow";
import { PILOT_SENSORS, type PilotSensor } from "../data/pilotSensors";
import { getNodeState, type NodeState } from "../lib/nodeState";

export interface MapNode extends PilotSensor {
    /** Dispositivo del backend enlazado por PK; null si el punto no está operativo. */
    device: Device | null;
    /** Alerta activa más grave del dispositivo, si la hay. */
    activeAlert: AlertItem | null;
    state: NodeState;
}

/** Puntos fijos del piloto con su estado real: comunicación del dispositivo y alertas activas. */
export function usePilotNodes(): MapNode[] {
    const { data: devices = [] } = useDevices();
    const { data: active } = useActiveAlerts();
    const now = useNow();

    return useMemo(() => {
        // El id del dispositivo del dashboard es el measuring_point_id de Alert Management.
        const activeByPoint = new Map<number, ActiveDevice>(
            (active?.devices ?? []).map((point) => [point.measuring_point_id, point])
        );
        return PILOT_SENSORS.map((sensor) => {
            const device = devices.find((candidate) => extractPk(candidate.name) === sensor.pkMeters) ?? null;
            const point = device ? (activeByPoint.get(device.id) ?? null) : null;
            const state = getNodeState({
                isLinked: device !== null,
                isOffline: device !== null && getDeviceStatus(device.lastConnection, now) === "offline",
                alerts: point,
            });
            return { ...sensor, device, activeAlert: point ? mostUrgentAlert(point.alerts) : null, state };
        });
    }, [devices, active, now]);
}
