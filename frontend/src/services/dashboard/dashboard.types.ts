export interface Device {
    id: number;
    name: string;
    lastConnection: string | null;
    lastBatteryLevel: number | null;
}