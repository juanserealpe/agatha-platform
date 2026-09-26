export type DeviceStatus = "online" | "delayed" | "offline";

// El sensor reporta ~cada 15 min; umbrales con margen de tolerancia
const ONLINE_THRESHOLD_MIN = 20;
const DELAYED_THRESHOLD_MIN = 120;

export function getDeviceStatus(lastConnection: string): DeviceStatus {
    const minutesSince = (Date.now() - new Date(lastConnection).getTime()) / 60000;
    if (minutesSince <= ONLINE_THRESHOLD_MIN) return "online";
    if (minutesSince <= DELAYED_THRESHOLD_MIN) return "delayed";
    return "offline";
}

export const DEVICE_STATUS_LABEL: Record<DeviceStatus, string> = {
    online: "En línea",
    delayed: "Retrasado",
    offline: "Sin conexión",
};

export const DEVICE_STATUS_DOT_CLASS: Record<DeviceStatus, string> = {
    online: "bg-green-500",
    delayed: "bg-yellow-500",
    offline: "bg-gray-400",
};