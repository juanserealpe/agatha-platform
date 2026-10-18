import type { AlertsService } from "./alerts.service";
import { emptyAlertsService } from "./emptyAlerts.adapter";

// Punto de composición: cuando exista el endpoint, sustituir por el adaptador HTTP aquí.
export const alertsService: AlertsService = emptyAlertsService;

export type { AlertsService } from "./alerts.service";
export type { Alert, AlertSeverity, AlertStatus, AlertType } from "./alerts.types";
