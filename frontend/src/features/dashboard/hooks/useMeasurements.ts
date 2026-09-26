import { useQuery } from "@tanstack/react-query";
import { dashboardService } from "@/services/dashboard/dashboard.service";

export function useMeasurements(deviceId: number | undefined, startDate: string, endDate: string) {
    return useQuery({
        queryKey: ["measurements", deviceId, startDate, endDate],
        queryFn: () => dashboardService.getMeasurements(deviceId!, startDate, endDate),
        enabled: deviceId !== undefined,
        staleTime: 10 * 60 * 1000, // el sensor reporta cada ~15 min, no tiene sentido refrescar antes
        gcTime: 30 * 60 * 1000,    // mantiene en caché rangos ya consultados al saltar entre ellos
    });
}