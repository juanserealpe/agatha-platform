import type { AlertManagementService } from "@/services/alertManagement";
import type { DashboardService } from "@/services/dashboard/dashboard.service";
import { ApiError } from "@/services/http/apiError";
import type { DemoAlertStore } from "./alertStore";
import { demoDevices } from "./demoData";
import { generateSeries } from "./series";

// Servicios de la demo con las mismas interfaces que los HTTP: los hooks y componentes no saben que no hay
// backend. Responden tras una espera corta para que la UI muestre sus estados de carga como con la red real.

const RESPONSE_DELAY_MS = 200;

export const EXPORT_UNAVAILABLE_MESSAGE = "La exportación a Excel no está disponible en la demo.";

function abortError(): DOMException {
    return new DOMException("La petición se canceló.", "AbortError");
}

/** Ejecuta `compute` tras la espera; respeta el `AbortSignal` como `fetch`. Un error de `compute` rechaza. */
function respond<T>(compute: () => T, signal?: AbortSignal): Promise<T> {
    return new Promise((resolve, reject) => {
        if (signal?.aborted) {
            reject(abortError());
            return;
        }
        const onAbort = () => {
            clearTimeout(timer);
            reject(abortError());
        };
        const timer = setTimeout(() => {
            signal?.removeEventListener("abort", onAbort);
            try {
                resolve(compute());
            } catch (error) {
                reject(error);
            }
        }, RESPONSE_DELAY_MS);
        signal?.addEventListener("abort", onAbort, { once: true });
    });
}

export function createDemoAlertManagementService(store: DemoAlertStore): AlertManagementService {
    return {
        search: (filters, page, signal) => respond(() => store.search(filters, page), signal),
        getDetail: (id, signal) => respond(() => store.getDetail(id), signal),
        getPending: (limit, signal) => respond(() => store.getPending(limit), signal),
        getActive: (signal) => respond(() => store.getActive(), signal),
        getDeviceAlerts: (deviceId, days, signal) => respond(() => store.getDeviceAlerts(deviceId, days), signal),
        classify: (id, request) => respond(() => store.classify(id, request)),
        label: (id, request) => respond(() => store.label(id, request)),
        addObservation: (id, request) => respond(() => store.addObservation(id, request)),
        exportXlsx: (_filters, signal) =>
            respond(() => {
                throw new ApiError({ status: 501, code: "DEMO_UNAVAILABLE", message: EXPORT_UNAVAILABLE_MESSAGE });
            }, signal),
    };
}

export function createDemoDashboardService(store: DemoAlertStore, now: () => Date = () => new Date()): DashboardService {
    return {
        getDevices: (signal) => respond(() => demoDevices(now()), signal),
        getSeries: (deviceId, startDate, endDate, variables, signal) =>
            respond(
                () =>
                    generateSeries({
                        deviceId,
                        startDate,
                        endDate,
                        variables,
                        intervals: store.levelIntervals(deviceId),
                        now: now(),
                    }),
                signal
            ),
    };
}
