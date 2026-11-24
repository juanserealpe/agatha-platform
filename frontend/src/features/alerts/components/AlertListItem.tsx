import type { CSSProperties } from "react";
import type { AlertId, AlertItem } from "@/services/alertManagement";
import { RelativeTime } from "@/shared/components/RelativeTime";
import { cn } from "@/lib/utils";
import {
    CLASSIFICATION_LABEL,
    displayLevel,
    EVENT_ICON,
    EVENT_LABEL,
    IN_PROGRESS_LABEL,
    levelStyle,
    MANAGEMENT_STATUS_STYLES,
} from "../config/alertManagementDisplay";
import { devicePkLabel } from "../lib/deviceLabel";
import { primaryManagementMode, type ManagementMode } from "../lib/managementMode";
import { LevelTag } from "./badges/LevelTag";

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

/**
 * Tarjeta del panel de alertas, con el mismo lenguaje que el aviso flotante (NewAlertToast): en curso, fondo
 * teñido del nivel con borde fino del mismo color. Finalizada (puede seguir sin clasificar): blanca y plana, pero
 * conserva el borde y el ícono del nivel, para que se reconozca qué fue sin competir con las que siguen en curso.
 */
export function AlertListItem({ alert, detail = "state", onSelect, onIntent }: AlertListItemProps) {
    const level = displayLevel(alert);
    const severity = levelStyle(level);
    const EventIcon = EVENT_ICON[alert.event];
    const stateLabel = alert.in_progress ? IN_PROGRESS_LABEL : "Finalizada";
    // En Activas importa qué se decidió ("Confirmada", "Falsa alerta"), más que si ya tiene etiqueta.
    const managementLabel = alert.classification
        ? CLASSIFICATION_LABEL[alert.classification]
        : MANAGEMENT_STATUS_STYLES.SIN_CLASIFICAR.label;
    const detailLabel = detail === "state" ? stateLabel : managementLabel;
    // Lo que le falta a alguien va en píldora; lo resuelto, como texto.
    const detailIsPending = detail === "state" ? alert.in_progress : alert.management_status === "SIN_CLASIFICAR";

    return (
        <button
            type="button"
            onClick={() => onSelect(String(alert.id), primaryManagementMode(alert.available_actions))}
            onPointerEnter={() => onIntent(alert.id)}
            onFocus={() => onIntent(alert.id)}
            // El pulso usa el color del nivel y solo anima la opacidad de un halo, nunca la del texto.
            style={{ "--glow-color": severity.hex } as CSSProperties}
            aria-label={`Alerta ${alert.code}, ${severity.label}, ${EVENT_LABEL[alert.event]}, ${stateLabel}, ${managementLabel}`}
            className={cn(
                "group flex w-full gap-2.5 rounded-xl border px-3 py-2 text-left transition-[background-color,box-shadow]",
                "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-gray-900",
                severity.hairlineClass,
                alert.in_progress
                    ? cn(severity.glassClass, "shadow-card hover:shadow-card-hover")
                    : "bg-white hover:bg-gray-50",
                alert.blinking && "alert-glow"
            )}
        >
            <span
                aria-hidden
                className={cn(
                    "mt-0.5 grid size-7 shrink-0 place-items-center rounded-lg ring-1 ring-black/5 ring-inset",
                    severity.iconClass,
                    alert.in_progress ? "bg-white/80" : severity.surfaceClass
                )}
            >
                <EventIcon size={14} strokeWidth={2.25} />
            </span>

            <span className="flex min-w-0 flex-1 flex-col">
                <span className="flex items-baseline gap-2">
                    <span className="truncate text-sm leading-snug font-semibold tracking-[-0.01em] text-gray-900">
                        {EVENT_LABEL[alert.event]}
                    </span>
                    <RelativeTime
                        value={alert.started_at}
                        className="ml-auto shrink-0 text-xs whitespace-nowrap text-gray-600"
                    />
                </span>

                <span className="truncate text-[13px] leading-snug text-gray-700" title={alert.device.pk}>
                    {devicePkLabel(alert.device.pk)} ·{" "}
                    <span className="font-data-mono text-xs text-gray-600">{alert.code}</span>
                </span>

                <span className="mt-1 flex items-center gap-2 text-xs font-medium text-gray-700">
                    <LevelTag level={level} />
                    <span
                        className={cn(
                            "ml-auto shrink-0 whitespace-nowrap",
                            detailIsPending
                                ? "rounded-full bg-white/65 px-2.5 py-0.5 font-semibold text-gray-900 ring-1 ring-black/5 transition-colors group-hover:bg-white/90"
                                : "text-gray-600"
                        )}
                    >
                        {detailLabel}
                    </span>
                </span>
            </span>
        </button>
    );
}
