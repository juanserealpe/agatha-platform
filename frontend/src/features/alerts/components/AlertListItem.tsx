import type { CSSProperties } from "react";
import type { AlertId, AlertItem } from "@/services/alertManagement";
import { RelativeTime } from "@/shared/components/RelativeTime";
import { cn } from "@/lib/utils";
import {
    CLASSIFICATION_LABEL,
    displayLevel,
    EVENT_LABEL,
    IN_PROGRESS_LABEL,
    levelStyle,
    MANAGEMENT_STATUS_STYLES,
} from "../config/alertManagementDisplay";
import { devicePkLabel } from "../lib/deviceLabel";
import { primaryManagementMode, type ManagementMode } from "../lib/managementMode";

interface AlertListItemProps {
    alert: AlertItem;
    /**
     * Dato de la esquina inferior derecha: si sigue en curso (lista de Sin clasificar, donde la gestión es la
     * misma en todas) o su estado de gestión (lista de Activas, donde todas están en curso).
     */
    detail?: "state" | "management";
    /** Se abre directo en la gestión pendiente (p. ej. clasificar), no en lectura. */
    onSelect: (alertId: string, mode: ManagementMode) => void;
    /** El usuario se acerca a la alerta: se precarga para que el modal abra sin espera. */
    onIntent: (alertId: AlertId) => void;
}

export function AlertListItem({ alert, detail = "state", onSelect, onIntent }: AlertListItemProps) {
    const severity = levelStyle(displayLevel(alert));
    const stateLabel = alert.in_progress ? IN_PROGRESS_LABEL : "Finalizada";
    // En Activas importa qué se decidió ("Confirmada", "Falsa alerta"), más que si ya tiene etiqueta.
    const managementLabel = alert.classification
        ? CLASSIFICATION_LABEL[alert.classification]
        : MANAGEMENT_STATUS_STYLES.SIN_CLASIFICAR.label;
    const detailLabel = detail === "state" ? stateLabel : managementLabel;
    // Resalta lo que le falta a alguien: en curso, o todavía sin clasificar.
    const detailIsPending = detail === "state" ? alert.in_progress : alert.management_status === "SIN_CLASIFICAR";

    // El pulso usa el color de la severidad (--glow-color) y solo anima la sombra, nunca la opacidad del texto.
    // Las finalizadas siguen pendientes de clasificar, pero se atenúan (fondo gris, borde fino) para que
    // resalten las que están en curso; el texto conserva su contraste.
    return (
        <button
            type="button"
            onClick={() => onSelect(String(alert.id), primaryManagementMode(alert.available_actions))}
            onPointerEnter={() => onIntent(alert.id)}
            onFocus={() => onIntent(alert.id)}
            style={{ "--glow-color": severity.hex } as CSSProperties}
            aria-label={`Alerta ${alert.code}, ${severity.label}, ${EVENT_LABEL[alert.event]}, ${stateLabel}, ${managementLabel}`}
            className={cn(
                "flex w-full flex-col gap-1 rounded-lg px-3 py-2 text-left transition-colors",
                "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-gray-900",
                severity.borderClass,
                alert.in_progress ? "border-2 bg-white hover:bg-gray-50" : "border bg-gray-50 hover:bg-gray-100",
                alert.blinking && "alert-glow"
            )}
        >
            <div className="flex items-center gap-2">
                <span className={cn("rounded-full px-2 py-0.5 text-xs font-semibold", severity.badgeClass)}>
                    {severity.label}
                </span>
                <span className={cn("text-sm font-semibold", alert.in_progress ? "text-gray-900" : "text-gray-700")}>
                    {EVENT_LABEL[alert.event]}
                </span>
                <RelativeTime value={alert.started_at} className="ml-auto shrink-0 font-data-mono text-xs text-gray-500" />
            </div>

            <p className="flex items-baseline gap-2 text-xs text-gray-500">
                <span className="min-w-0 truncate font-data-mono" title={alert.device.pk}>
                    {devicePkLabel(alert.device.pk)} · {alert.code}
                </span>
                <span className={cn("ml-auto shrink-0 whitespace-nowrap", detailIsPending && "font-semibold text-gray-700")}>
                    {detailLabel}
                </span>
            </p>
        </button>
    );
}
