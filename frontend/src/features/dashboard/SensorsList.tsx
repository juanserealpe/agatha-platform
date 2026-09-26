import { getBatteryVisual } from "@/theme/severity";
import type { Device } from "@/services/dashboard/dashboard.types";
import {
    getDeviceStatus,
    DEVICE_STATUS_LABEL,
    DEVICE_STATUS_DOT_CLASS,
} from "./sensorStatus";

interface SensorsListProps {
    devices: Device[];
    onSensorClick: (device: Device) => void;
    selectedDeviceId?: number;
}

export function SensorsList({ devices, onSensorClick, selectedDeviceId }: SensorsListProps) {
    return (
        <div className="flex flex-col gap-2 h-full">
            <h3 className="text-sm font-bold text-gray-900 px-1">
                Sensores ({devices.length})
            </h3>
            <div className="flex flex-col gap-2 overflow-y-auto">
                {devices.map((device) => {
                    const status = getDeviceStatus(device.lastConnection);
                    const battery = getBatteryVisual(device.lastBatteryLevel ?? 0);
                    const isSelected = device.id === selectedDeviceId;

                    return (
                        <button
                            key={device.id}
                            type="button"
                            onClick={() => onSensorClick(device)}
                            className={`text-left border rounded-lg p-3 flex items-center gap-3 transition-colors cursor-pointer hover:brightness-95 ${isSelected ? "ring-2 ring-blue-400 bg-blue-50" : "border-gray-200"
                                }`}
                        >
                            <span className={`w-2.5 h-2.5 rounded-full shrink-0 ${DEVICE_STATUS_DOT_CLASS[status]}`} />

                            <div className="flex-1 min-w-0">
                                <p className="text-sm font-medium text-gray-900 truncate">{device.name}</p>
                                <p className="text-xs text-gray-400">
                                    {DEVICE_STATUS_LABEL[status]} <br /> Última comunicación:{" "}
                                    {device.lastConnection ? new Date(device.lastConnection).toLocaleString() : "Nunca"}
                                </p>
                            </div>

                            <div className={`flex items-center gap-1 shrink-0 ${battery.colorClass}`}>
                                <battery.Icon size={16} />
                                <span className="font-data-mono text-xs">{device.lastBatteryLevel ?? "—"}%</span>
                            </div>
                        </button>
                    );
                })}
            </div>
        </div>
    );
}