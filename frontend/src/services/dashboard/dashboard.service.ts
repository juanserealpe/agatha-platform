import type { Device } from "./dashboard.types";

const BASE_URL = import.meta.env.DEV ? "" : import.meta.env.VITE_API_BASE_URL;

export const dashboardService = {
    async getDevices(): Promise<Device[]> {
        const res = await fetch(`${BASE_URL}/api/dashboard/devices`);
        if (!res.ok) {
            throw new Error(`No se pudo obtener el listado de dispositivos (${res.status})`);
        }
        return res.json();
    },
};