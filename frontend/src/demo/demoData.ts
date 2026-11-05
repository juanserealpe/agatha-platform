import type { AlertLevel } from "@/services/alertManagement";
import type { Device } from "@/services/dashboard/dashboard.types";
import type { LeakThresholds } from "@/services/dashboard/series.types";

// Datos fijos de la demo. Los tres puntos son los del piloto (features/map/data/pilotSensors.ts, enlazados por
// el PK del nombre), con ids de dispositivo propios de la demo: nunca se confunden con los reales.

export interface DemoPoint {
    id: number;
    name: string;
    batteryLevel: number;
}

export const DEMO_POINTS = {
    punto1: { id: 9001, name: "Sensor MP-9001 PK34-470", batteryLevel: 87 },
    punto2: { id: 9002, name: "Sensor MP-9002 PK38-174", batteryLevel: 92 },
    punto3: { id: 9003, name: "Sensor MP-9003 PK39-268", batteryLevel: 78 },
} as const satisfies Record<string, DemoPoint>;

/** El escenario guiado ocurre en el Punto 1 (PK 34+470). */
export const SCENARIO_POINT_ID = DEMO_POINTS.punto1.id;

/** La primera alerta de la demo es la 900001 (ALR-900001). */
export const FIRST_DEMO_ALERT_ID = 900001;

// Misma plantilla que DEFAULT_LEAK_THRESHOLDS (features/telemetry): la serie del indicador no trae `thresholds`
// y la gráfica dibuja esas franjas, así que los valores simulados deben caer en ellas. Una prueba lo verifica.
export const DEMO_THRESHOLDS: LeakThresholds = { amarillo: 0.16, naranja: 0.25, rojo: 0.35 };

/** Índice de ruido simulado en cada nivel (dentro de su franja) y en normalidad. */
export const INDICATOR_BY_LEVEL: Record<AlertLevel | "NORMAL", number> = {
    NORMAL: 0.08,
    AMARILLO: 0.21,
    NARANJA: 0.3,
    ROJO: 0.42,
};

/**
 * Horas hacia atrás en que empieza cada nivel respecto al paso que lo dispara. El escenario avanza en segundos,
 * pero el backend abre y sube alertas tras horas de ruido sostenido: correr el inicio hace que la subida se vea
 * en las gráficas (rango de 1 semana) y que la línea de tiempo cuente horas creíbles.
 */
export const LEVEL_LEAD_HOURS: Record<AlertLevel, number> = {
    AMARILLO: 9,
    NARANJA: 5,
    ROJO: 2,
};

/** Si vuelve antes de esto y sigue sin clasificar, se reactiva la misma alerta (texto del mensaje de fin). */
export const REACTIVATION_WINDOW_HOURS = 24;

export function demoDevices(now: Date): Device[] {
    return Object.values(DEMO_POINTS).map((point) => ({
        id: point.id,
        name: point.name,
        lastConnection: now.toISOString(),
        lastBatteryLevel: point.batteryLevel,
    }));
}

export function findDemoPoint(id: number): DemoPoint | undefined {
    return Object.values(DEMO_POINTS).find((point) => point.id === id);
}
