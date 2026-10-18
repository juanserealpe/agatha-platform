import type { Device } from "@/services/dashboard/dashboard.types";
import { RelativeTime } from "@/shared/components/RelativeTime";
import { cn } from "@/lib/utils";
import { getBatteryVisual } from "../lib/battery";
import { DEVICE_STATUS_DOT_CLASS, DEVICE_STATUS_LABEL, type DeviceStatus } from "../lib/sensorStatus";

interface SensorListItemProps {
    device: Device;
    status: DeviceStatus;
    isSelected: boolean;
    onSelect: (deviceId: number) => void;
}

export function SensorListItem({ device, status, isSelected, onSelect }: SensorListItemProps) {
    const battery = getBatteryVisual(device.lastBatteryLevel);

    return (
        <button
            type="button"
            onClick={() => onSelect(device.id)}
            aria-pressed={isSelected}
            className={cn(
                "flex w-full items-center gap-3 rounded-lg border p-3 text-left transition-colors",
                "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-gray-900",
                isSelected ? "border-blue-400 bg-blue-50 ring-1 ring-blue-400" : "border-gray-200 hover:bg-gray-50"
            )}
        >
            <span className={cn("h-2.5 w-2.5 shrink-0 rounded-full", DEVICE_STATUS_DOT_CLASS[status])} aria-hidden />

            <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-medium text-gray-900">{device.name}</p>
                <p className="text-xs text-gray-500">
                    {DEVICE_STATUS_LABEL[status]} · <RelativeTime value={device.lastConnection} />
                </p>
            </div>

            <div
                className={cn("flex shrink-0 items-center gap-1", battery.colorClass)}
                title={battery.isLow ? "Batería baja" : "Batería"}
            >
                <battery.Icon size={16} aria-hidden />
                <span className="font-data-mono text-xs">{battery.label}</span>
            </div>
        </button>
    );
}
