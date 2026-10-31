import type { CSSProperties } from "react";
import type { AlertItem } from "@/services/alertManagement";
import { RelativeTime } from "@/shared/components/RelativeTime";
import { cn } from "@/lib/utils";
import { displayLevel, EVENT_LABEL, IN_PROGRESS_LABEL, levelStyle } from "../config/alertManagementDisplay";
import { devicePkLabel } from "../lib/deviceLabel";

interface AlertListItemProps {
    alert: AlertItem;
    onSelect: (alertId: string) => void;
}

export function AlertListItem({ alert, onSelect }: AlertListItemProps) {
    const severity = levelStyle(displayLevel(alert));
    const stateLabel = alert.in_progress ? IN_PROGRESS_LABEL : "Finalizada";

    // El pulso usa el color de la severidad (--glow-color) y solo anima la sombra, nunca la opacidad del texto.
    return (
        <button
            type="button"
            onClick={() => onSelect(String(alert.id))}
            style={{ "--glow-color": severity.hex } as CSSProperties}
            aria-label={`Alerta ${alert.code}, ${severity.label}, ${EVENT_LABEL[alert.event]}, ${stateLabel}`}
            className={cn(
                "flex w-full flex-col gap-1 rounded-lg border-2 bg-white px-3 py-2 text-left transition-colors",
                "hover:bg-gray-50 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-gray-900",
                severity.borderClass,
                alert.blinking && "alert-glow"
            )}
        >
            <div className="flex items-center gap-2">
                <span className={cn("rounded-full px-2 py-0.5 text-xs font-semibold", severity.badgeClass)}>
                    {severity.label}
                </span>
                <span className="text-sm font-semibold text-gray-900">{EVENT_LABEL[alert.event]}</span>
                <RelativeTime value={alert.started_at} className="ml-auto shrink-0 font-data-mono text-xs text-gray-500" />
            </div>

            <p className="flex items-baseline gap-2 text-xs text-gray-500">
                <span className="min-w-0 truncate font-data-mono" title={alert.device.pk}>
                    {devicePkLabel(alert.device.pk)} · {alert.code}
                </span>
                <span className={cn("ml-auto shrink-0 whitespace-nowrap", alert.in_progress && "font-semibold text-gray-700")}>
                    {stateLabel}
                </span>
            </p>
        </button>
    );
}
