export interface Device {
    id: number;
    name: string;
    lastConnection: string; // ISO 8601
    lastBatteryLevel: number; // 0-100
}