import { setBrowserNotificationTitlePrefix } from "@/features/alerts";
import { installAlertManagementService } from "@/services/alertManagement";
import { installDashboardService } from "@/services/dashboard/dashboard.service";
import { blockBackendRequests } from "@/services/http";
import { installEventSourceFactory } from "@/services/realtime";
import { createDemoAlertStore, type DemoAlertStore } from "./alertStore";
import { createDemoStreamController, type DemoStreamController } from "./demoEventSource";
import { createDemoAlertManagementService, createDemoDashboardService } from "./demoServices";
import { createDemoScenario, type DemoScenario } from "./scenario";

const BACKEND_BLOCKED_MESSAGE = "La demo no se conecta al backend: este dato no está simulado.";
const NOTIFICATION_TITLE_PREFIX = "[SIMULACIÓN] ";

/** Lo que el panel de la demo necesita para manejar la simulación. */
export interface DemoController {
    store: DemoAlertStore;
    scenario: DemoScenario;
    stream: DemoStreamController;
}

/**
 * Cambia todos los accesos al backend por los simulados. Se llama una sola vez, antes del primer render: los
 * puntos de composición no se cambian en caliente. Bloquea además toda petición HTTP, para que un servicio sin
 * simular falle a la vista en vez de traer datos reales.
 */
export function installDemo(): DemoController {
    const store = createDemoAlertStore();
    const stream = createDemoStreamController(store);

    installAlertManagementService(createDemoAlertManagementService(store));
    installDashboardService(createDemoDashboardService(store));
    installEventSourceFactory(stream.createEventSource);
    blockBackendRequests(BACKEND_BLOCKED_MESSAGE);
    setBrowserNotificationTitlePrefix(NOTIFICATION_TITLE_PREFIX);

    return { store, scenario: createDemoScenario(store), stream };
}
