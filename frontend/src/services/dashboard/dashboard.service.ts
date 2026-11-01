import type { SeriesResponse } from "@/services/dashboard/series.types";
import { apiPath, httpClient } from "@/services/http";
import type { Device } from "./dashboard.types";

export interface DashboardService {
    getDevices(signal?: AbortSignal): Promise<Device[]>;
    getSeries(
        deviceId: number,
        startDate: string,
        endDate: string,
        variables: string[],
        signal?: AbortSignal
    ): Promise<SeriesResponse>;
}

const httpDashboardService: DashboardService = {
    getDevices(signal?: AbortSignal): Promise<Device[]> {
        return httpClient.get(apiPath("api", "dashboard", "devices"), {
            signal,
            errorMessage: "No se pudo obtener el listado de dispositivos.",
        });
    },

    getSeries(
        deviceId: number,
        startDate: string,
        endDate: string,
        variables: string[],
        signal?: AbortSignal
    ): Promise<SeriesResponse> {
        return httpClient.get(apiPath("api", "dashboard", "series"), {
            query: { deviceId, startDate, endDate, variables: variables.join(",") },
            signal,
            errorMessage: "No se pudieron obtener las series.",
        });
    },
};

// Punto de composición: los hooks usan esta instancia.
export let dashboardService: DashboardService = httpDashboardService;

/** Reemplaza la implementación (la demo usa una en memoria). Solo antes del primer render: no se cambia en caliente. */
export function installDashboardService(service: DashboardService) {
    dashboardService = service;
}
