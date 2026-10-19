import type { CSSProperties } from "react";
import type { Alert } from "@/services/alerts";
import { formatPk } from "@/features/sensors";
import { RelativeTime } from "@/shared/components/RelativeTime";
import { cn } from "@/lib/utils";
import { ALERT_STATUS_LABEL, ALERT_TYPE_LABEL, SEVERITY_STYLES, shouldPulse } from "../config/alertDisplay";

interface AlertListItemProps {
    alert: Alert;
    onSelect: (alertId: string) => void;
}

export function AlertListItem({ alert, onSelect }: AlertListItemProps) {
    const severity = SEVERITY_STYLES[alert.severity];
    const isUnclassified = alert.status === "unclassified";

    // El pulso usa el color de la severidad (--glow-color) y solo anima la sombra, nunca la opacidad del texto.
    return (
        <button
            type="button"
            onClick={() => onSelect(alert.id)}
            style={{ "--glow-color": severity.hex } as CSSProperties}
            aria-label={`Alerta ${severity.label} de ${ALERT_TYPE_LABEL[alert.type]}, ${ALERT_STATUS_LABEL[alert.status]}`}
            className={cn(
                "flex w-full flex-col gap-1 rounded-lg border-2 px-3 py-2 text-left transition-colors",
                "hover:bg-gray-50 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-gray-900",
                severity.borderClass,
                isUnclassified ? "bg-white" : "bg-gray-50",
                shouldPulse(alert) && "alert-glow"
            )}
        >
            <div className="flex items-center gap-2">
                <span className={cn("rounded-full px-2 py-0.5 text-xs font-semibold", severity.badgeClass)}>
                    {severity.label}
                </span>
                <span className="text-sm font-semibold text-gray-900">{ALERT_TYPE_LABEL[alert.type]}</span>
                <RelativeTime value={alert.detectedAt} className="ml-auto shrink-0 font-data-mono text-xs text-gray-500" />
            </div>

            <p className="flex items-baseline gap-2 text-xs text-gray-500">
                <span className="min-w-0 truncate font-data-mono">
                    Disp. {alert.deviceId}
                    {alert.pkMeters !== null && <> · {formatPk(alert.pkMeters).replace("PK ", "PK ")}</>}
                </span>
                <span className={cn("ml-auto shrink-0 whitespace-nowrap", isUnclassified && "font-semibold text-gray-700")}>
                    {ALERT_STATUS_LABEL[alert.status]}
                </span>
            </p>
        </button>
    );
}
