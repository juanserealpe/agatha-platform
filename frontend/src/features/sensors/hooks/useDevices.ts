import { useQuery } from "@tanstack/react-query";
import { dashboardService } from "@/services/dashboard/dashboard.service";

export function useDevices() {
    return useQuery({
        queryKey: ["devices"],
        queryFn: ({ signal }) => dashboardService.getDevices(signal),
        refetchInterval: 60_000, // refresco periódico; ajusta según necesites
    });
}