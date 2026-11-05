import type { Device } from "@/services/dashboard/dashboard.types";
import { RelativeTime } from "@/shared/components/RelativeTime";
import { cn } from "@/lib/utils";
import { getBatteryVisual } from "../lib/battery";
import { DEVICE_STATUS_DOT_CLASS, DEVICE_STATUS_LABEL, type DeviceStatus, type SensorAlertIndicator } from "../lib/sensorStatus";

interface SensorListItemProps {
    device: Device;
    status: DeviceStatus;
    /** Alerta activa del sensor; null si no tiene. */
    alert: SensorAlertIndicator | null;
    isSelected: boolean;
    onSelect: (deviceId: number) => void;
}

export function SensorListItem({ device, status, alert, isSelected, onSelect }: SensorListItemProps) {
    const battery = getBatteryVisual(device.lastBatteryLevel);

    return (
        <button
            type="button"
            onClick={() => onSelect(device.id)}
            aria-pressed={isSelected}
            className={cn(
                "flex w-full items-center gap-3 rounded-lg border p-3 text-left transition-colors",
                "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-gray-900",
                isSelected
                    ? "border-blue-400 bg-blue-50 ring-1 ring-blue-400"
                    : cn("border-gray-200", alert ? alert.surfaceClass : "hover:bg-gray-50")
            )}
        >
            {/* Con alerta activa manda la criticidad (punto con halo); la comunicación sigue en el texto. */}
            <span className="relative flex h-2.5 w-2.5 shrink-0" aria-hidden>
                {alert?.isBlinking && <span className={cn("sensor-alert-pulse absolute inset-0 rounded-full", alert.dotClass)} />}
                <span
                    className={cn(
                        "relative h-2.5 w-2.5 rounded-full",
                        alert ? cn(alert.dotClass, alert.haloClass) : DEVICE_STATUS_DOT_CLASS[status]
                    )}
                />
            </span>

            <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-medium text-gray-900">{device.name}</p>
                <p className="flex flex-wrap items-center gap-x-1.5 text-xs text-gray-500">
                    {alert && (
                        <span className={cn("rounded-full px-1.5 py-px text-[11px] font-semibold", alert.badgeClass)}>
                            {alert.label}
                        </span>
                    )}
                    <span>
                        {DEVICE_STATUS_LABEL[status]} · <RelativeTime value={device.lastConnection} />
                    </span>
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
