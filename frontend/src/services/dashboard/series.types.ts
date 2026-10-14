//El contrato con el backend (forma del JSON)
export interface SeriesPoint {
    timestamp: string;
    value: number;
}

export interface Series {
    key: string; // "b1", "b2", "densidad", "indicador"
    name: string; // nombre legible que manda el backend
    unit: string; // "-" mientras no esté definida
    source: "measurement" | "evaluation"; // crudo del sensor o calculado
    data: SeriesPoint[];
}

export interface SeriesResponse {
    deviceId: number;
    startDate: string;
    endDate: string;
    series: Series[];
}