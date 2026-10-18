// Puntos de instalación del piloto (fijos durante el piloto, no se consumen del backend).
// Cada punto se enlaza con un dispositivo del backend cuando el nombre de este contiene el mismo PK;
// los puntos sin dispositivo enlazado se muestran como no operativos.

export type PipeMaterial = "steel" | "flexible";

export interface PilotSensor {
    id: string;
    label: string;
    pkMeters: number;
    position: [number, number];
    pipeMaterial: PipeMaterial;
}

export const PILOT_SENSORS: PilotSensor[] = [
    { id: "punto-1", label: "Punto 1", pkMeters: 34470, position: [6.032335, -73.561075], pipeMaterial: "flexible" },
    { id: "punto-2", label: "Punto 2", pkMeters: 38174, position: [6.030825, -73.537625], pipeMaterial: "steel" },
    { id: "punto-3", label: "Punto 3", pkMeters: 39268, position: [6.036492, -73.531222], pipeMaterial: "steel" },
];
