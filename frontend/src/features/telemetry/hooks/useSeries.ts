//Caché, carga y refresco automático
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { dashboardService } from "@/services/dashboard/dashboard.service";


export function useSeries(
    deviceId: number | undefined,
    startDate: string,
    endDate: string,
    variables: string[]
) {
    return useQuery({
        queryKey: ["series", deviceId, startDate, endDate, variables],
        queryFn: () => dashboardService.getSeries(deviceId!, startDate, endDate, variables),
        enabled: deviceId !== undefined,
        staleTime: 60_000,
        gcTime: 30 * 60 * 1000,
        placeholderData: keepPreviousData, // refresco automático; B1/B2 llegan cada 15 min
    });
}