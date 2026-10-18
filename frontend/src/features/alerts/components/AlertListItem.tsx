import type { Alert } from "@/services/alerts";
import { formatPk } from "@/features/sensors";
import { RelativeTime } from "@/shared/components/RelativeTime";
import { cn } from "@/lib/utils";
import { ALERT_STATUS_LABEL, ALERT_TYPE_LABEL, SEVERITY_STYLES } from "../config/alertDisplay";

interface AlertListItemProps {
    alert: Alert;
    onSelect?: (alertId: string) => void;
}

export function AlertListItem({ alert, onSelect }: AlertListItemProps) {
    const severity = SEVERITY_STYLES[alert.severity];
    const isUnclassified = alert.status === "unclassified";

    return (
        <button
            type="button"
            onClick={() => onSelect?.(alert.id)}
            disabled={!onSelect}
            aria-label={`Alerta ${severity.label} de ${ALERT_TYPE_LABEL[alert.type]}, ${ALERT_STATUS_LABEL[alert.status]}`}
            className={cn(
                "flex w-full flex-col gap-1 rounded-lg border border-l-4 border-gray-200 p-3 text-left transition-colors",
                "enabled:hover:bg-gray-50 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-gray-900",
                severity.accentClass,
                isUnclassified ? "bg-white" : "bg-gray-50"
            )}
        >
            <div className="flex items-center gap-2">
                <span className={cn("rounded-full px-2 py-0.5 text-xs font-semibold", severity.badgeClass)}>
                    {severity.label}
                </span>
                <span className="text-sm font-semibold text-gray-900">{ALERT_TYPE_LABEL[alert.type]}</span>
                <RelativeTime value={alert.detectedAt} className="ml-auto shrink-0 font-data-mono text-xs text-gray-500" />
            </div>

            <p className="font-data-mono text-xs text-gray-500">
                {alert.pkMeters !== null && <>{formatPk(alert.pkMeters)} · </>}
                <span className={isUnclassified ? "font-semibold text-gray-700" : undefined}>
                    {ALERT_STATUS_LABEL[alert.status]}
                </span>
            </p>
        </button>
    );
}
