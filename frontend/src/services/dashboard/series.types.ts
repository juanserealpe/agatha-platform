//El contrato con el backend (forma del JSON)
export interface SeriesPoint {
    timestamp: string;
    value: number;
}

/** Cortes de I(24 h) del Rule Engine, escala 0–1. */
export interface LeakThresholds {
    amarillo: number;
    naranja: number;
    rojo: number;
}

export interface Series {
    key: string; // "b1", "b2", "densidad", "indicador"
    name: string; // nombre legible que manda el backend
    unit: string; // "-" mientras no esté definida
    source: "measurement" | "evaluation"; // crudo del sensor o calculado
    data: SeriesPoint[];
    // Pendiente en backend (rule_engine README: "campo `thresholds` en la serie del indicador").
    // Forma supuesta, la misma del evento leak_threshold_exceeded; ajustar cuando se publique.
    thresholds?: LeakThresholds;
}

export interface SeriesResponse {
    deviceId: number;
    startDate: string;
    endDate: string;
    series: Series[];
}