import type { Alert } from "./alerts.types";

/** Contrato de acceso a alertas. Los hooks dependen de esta interfaz, no de una implementación. */
export interface AlertsService {
    /** Alertas más recientes primero. */
    getRecent(limit: number): Promise<Alert[]>;
    /** Alertas sin clasificar, más recientes primero. */
    getPending(): Promise<Alert[]>;
    /** null si la alerta no existe. */
    getById(id: string): Promise<Alert | null>;
}
