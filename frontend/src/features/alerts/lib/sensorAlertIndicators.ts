import type { SensorAlertIndicator } from "@/features/sensors";
import type { ActiveResponse } from "@/services/alertManagement";
import { LEVEL_ALERT_LABEL, levelStyle } from "../config/alertManagementDisplay";

/**
 * Indicador de alerta por id de dispositivo, para la lista de sensores. Usa el resumen por punto que ya calcula el
 * backend (nivel mayor y si alguna titila), igual que el mapa. Un punto sale de `GET /active` cuando todas sus
 * alertas finalizan, así que el sensor vuelve solo a su color normal.
 */
export function sensorAlertIndicators(data: ActiveResponse | undefined): ReadonlyMap<number, SensorAlertIndicator> {
    // El id del dispositivo del dashboard es el measuring_point_id de Alert Management.
    return new Map(
        (data?.devices ?? []).map((point) => {
            const { dotClass, haloClass, surfaceClass, badgeClass } = levelStyle(point.level);
            return [
                point.measuring_point_id,
                { label: LEVEL_ALERT_LABEL[point.level], dotClass, haloClass, surfaceClass, badgeClass, isBlinking: point.blinking },
            ];
        })
    );
}
