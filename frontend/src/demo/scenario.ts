import type { AlertId } from "@/services/alertManagement";
import type { DemoAlertStore } from "./alertStore";
import { SCENARIO_POINT_ID } from "./demoData";

// Escenario guiado de la demo sobre el Punto 1: Amarilla → Naranja → Roja (titila) → clasificar → Normal.
// Cada paso se aplica al store, que emite los mismos eventos que el backend.

export type ScenarioStepId = "AMARILLO" | "NARANJA" | "ROJO" | "CLASIFICAR" | "NORMAL";

export interface ScenarioStep {
    id: ScenarioStepId;
    title: string;
    description: string;
}

export const SCENARIO_STEPS: readonly ScenarioStep[] = [
    {
        id: "AMARILLO",
        title: "Alerta nueva en Amarillo",
        description: "El ruido del Punto 1 lleva horas por encima de lo normal y se abre una alerta.",
    },
    {
        id: "NARANJA",
        title: "Sube a Naranja",
        description: "El ruido sigue subiendo: la misma alerta pasa a Naranja y llega un aviso nuevo.",
    },
    {
        id: "ROJO",
        title: "Sube a Rojo",
        description: "La alerta llega a Rojo y el marcador titila hasta que alguien la clasifique.",
    },
    {
        id: "CLASIFICAR",
        title: "Clasificar",
        description: "Se confirma la alerta (o se clasifica desde el detalle) y deja de titilar.",
    },
    {
        id: "NORMAL",
        title: "Vuelve a lo normal",
        description: "El ruido baja, la alerta termina y el punto vuelve a Normal.",
    },
];

export interface DemoScenario {
    /** Índice del próximo paso; `SCENARIO_STEPS.length` cuando terminó. */
    position(): number;
    nextStep(): ScenarioStep | null;
    /** Aplica el próximo paso y lo devuelve; null si el escenario ya terminó. */
    advance(): ScenarioStep | null;
    /** Borra las alertas simuladas y vuelve al primer paso. */
    reset(): void;
}

export function createDemoScenario(store: DemoAlertStore): DemoScenario {
    let position = 0;
    let alertId: AlertId | null = null;

    // Un paso puede no tener efecto (p. ej. la alerta ya se clasificó desde el modal): igual cuenta como hecho.
    const apply: Record<ScenarioStepId, () => void> = {
        AMARILLO: () => {
            alertId = store.openAlert("AMARILLO", SCENARIO_POINT_ID)?.id ?? store.activeAlertId(SCENARIO_POINT_ID);
        },
        NARANJA: () => {
            if (alertId !== null) store.raiseLevel(alertId);
        },
        ROJO: () => {
            if (alertId !== null) store.raiseLevel(alertId);
        },
        CLASIFICAR: () => {
            if (alertId === null || store.getDetail(alertId).classification !== null) return;
            store.classify(alertId, { classification: "CONFIRMADA", origin: "WEB" });
        },
        NORMAL: () => {
            if (alertId !== null) store.finishAlert(alertId);
        },
    };

    return {
        position: () => position,
        nextStep: () => SCENARIO_STEPS[position] ?? null,
        advance() {
            const step = SCENARIO_STEPS[position];
            if (!step) return null;
            apply[step.id]();
            position++;
            return step;
        },
        reset() {
            store.reset();
            position = 0;
            alertId = null;
        },
    };
}
