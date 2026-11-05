export type DeviceStatus = "online" | "delayed" | "offline";

// El sensor reporta ~cada 15 min; umbrales con margen de tolerancia
const ONLINE_THRESHOLD_MIN = 20;
const DELAYED_THRESHOLD_MIN = 120;

export function getDeviceStatus(
    lastConnection: string | null,
    now: number = Date.now()
): DeviceStatus {
    if (!lastConnection) return "offline";
    const minutesSince = (now - new Date(lastConnection).getTime()) / 60000;
    if (minutesSince <= ONLINE_THRESHOLD_MIN) return "online";
    if (minutesSince <= DELAYED_THRESHOLD_MIN) return "delayed";
    return "offline";
}

export const DEVICE_STATUS_LABEL: Record<DeviceStatus, string> = {
    online: "En línea",
    delayed: "Retrasado",
    offline: "Sin comunicación",
};

// Estado de comunicación, no de alerta: se evita la escala amarillo/naranja/rojo, reservada a severidad.
export const DEVICE_STATUS_DOT_CLASS: Record<DeviceStatus, string> = {
    online: "bg-emerald-500",
    delayed: "border-2 border-gray-400 bg-white",
    offline: "bg-gray-400",
};

/**
 * Alerta activa de un sensor: mientras dure, el punto (con halo), el fondo y una píldora toman su color de
 * criticidad (igual que en el mapa) y vuelve al de comunicación cuando la alerta finaliza. La arma la página con las alertas activas; esta
 * feature no conoce Alert Management.
 */
export interface SensorAlertIndicator {
    /** P. ej. "Alerta roja". */
    label: string;
    dotClass: string;
    /** Anillo suave alrededor del punto. */
    haloClass: string;
    /** Fondo tenue de la tarjeta. */
    surfaceClass: string;
    /** Píldora con el nombre del nivel. */
    badgeClass: string;
    /** Roja sin clasificar: el halo pulsa (en el mapa, el punto titila). */
    isBlinking: boolean;
}
