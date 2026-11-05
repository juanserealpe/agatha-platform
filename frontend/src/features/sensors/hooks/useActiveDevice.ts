import { useState } from "react";
import { useDevices } from "./useDevices";

/**
 * Sensor activo para la telemetría: el elegido por el usuario o, por defecto, el primero de la lista.
 * Se guarda el id (no el objeto) para que los refrescos de la query muestren siempre datos actuales.
 */
export function useActiveDevice() {
    const { data: devices = [] } = useDevices();
    const [selectedId, setSelectedId] = useState<number | null>(null);

    const activeDevice = devices.find((device) => device.id === selectedId) ?? devices[0] ?? null;

    return { activeDevice, selectDevice: setSelectedId };
}
