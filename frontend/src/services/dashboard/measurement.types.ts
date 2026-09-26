export interface MeasurementPoint {
    timestamp: string;
    value: number;
}

export interface MetricMeasurements {
    metricId: number;
    metricName: string;
    unit: string;
    data: MeasurementPoint[];
}

export interface MeasurementsResponse {
    deviceId: number;
    startDate: string;
    endDate: string;
    measurements: MetricMeasurements[];
}