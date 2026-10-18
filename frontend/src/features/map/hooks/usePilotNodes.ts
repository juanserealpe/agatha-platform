import { useMemo } from "react";
import { usePendingAlerts } from "@/features/alerts";
import { extractPk, getDeviceStatus, useDevices } from "@/features/sensors";
import type { Alert } from "@/services/alerts";
import type { Device } from "@/services/dashboard/dashboard.types";
import { useNow } from "@/shared/hooks/useNow";
import { PILOT_SENSORS, type PilotSensor } from "../data/pilotSensors";
import { getNodeState, pickMostSevereAlert, type NodeState } from "../lib/nodeState";

export interface MapNode extends PilotSensor {
    /** Dispositivo del backend enlazado por PK; null si el punto no está operativo. */
    device: Device | null;
    /** Alerta vigente más grave del dispositivo, si la hay. */
    activeAlert: Alert | null;
    state: NodeState;
}

/** Puntos fijos del piloto con su estado real: comunicación del dispositivo y alertas pendientes. */
export function usePilotNodes(): MapNode[] {
    const { data: devices = [] } = useDevices();
    // Solo hay consulta de alertas pendientes; cuando el backend exponga "alertas activas" se usará esa.
    const { data: pendingAlerts = [] } = usePendingAlerts();
    const now = useNow();

    return useMemo(
        () =>
            PILOT_SENSORS.map((sensor) => {
                const device = devices.find((candidate) => extractPk(candidate.name) === sensor.pkMeters) ?? null;
                const alerts = device ? pendingAlerts.filter((alert) => alert.deviceId === device.id) : [];
                const state = getNodeState({
                    isLinked: device !== null,
                    isOffline: device !== null && getDeviceStatus(device.lastConnection, now) === "offline",
                    alerts,
                });
                return { ...sensor, device, activeAlert: pickMostSevereAlert(alerts), state };
            }),
        [devices, pendingAlerts, now]
    );
}
