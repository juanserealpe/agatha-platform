import type { CSSProperties } from "react";
import { ArrowRight, BellRing, X } from "lucide-react";
import { RelativeTime } from "@/shared/components/RelativeTime";
import { cn } from "@/lib/utils";
import { displayLevel, EVENT_LABEL, levelStyle } from "../config/alertManagementDisplay";
import { useAlertDetailParam } from "../hooks/useAlertDetailParam";
import { useNewAlerts } from "../hooks/useNewAlerts";
import { mostUrgentAlert } from "../lib/alertPriority";
import { devicePkLabel } from "../lib/deviceLabel";

/**
 * Aviso de alerta nueva. No se cierra solo: en monitoreo una alerta no debe desaparecer sin que alguien la vea.
 * Si llegan varias, muestra la más grave y cuántas más hay.
 */
export function NewAlertToast() {
    const { unseen, dismiss } = useNewAlerts();
    const { open } = useAlertDetailParam();
    const alert = mostUrgentAlert(unseen);

    if (!alert) return null;

    const level = displayLevel(alert);
    const severity = levelStyle(level);
    const others = unseen.length - 1;
    const isUrgent = level === "ROJO";

    return (
        <div
            role={isUrgent ? "alert" : "status"}
            aria-live={isUrgent ? "assertive" : "polite"}
            style={{ "--glow-color": severity.hex } as CSSProperties}
            className={cn(
                "w-80 max-w-[calc(100%-1.5rem)] rounded-xl border-2 bg-white p-3 shadow-xl",
                severity.borderClass,
                alert.blinking && "alert-glow"
            )}
        >
            <div className="flex items-center gap-2">
                <BellRing size={16} className="shrink-0 text-gray-700" aria-hidden />
                <p className="text-sm font-bold text-gray-900">Nueva alerta</p>
                <span className={cn("rounded-full px-2 py-0.5 text-xs font-semibold", severity.badgeClass)}>
                    {severity.label}
                </span>
                {others > 0 && (
                    <span className="whitespace-nowrap rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-700">
                        +{others} {others === 1 ? "nueva" : "nuevas"}
                    </span>
                )}
                <button
                    type="button"
                    onClick={() => dismiss(alert.id)}
                    aria-label="Descartar aviso"
                    className="ml-auto rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-700"
                >
                    <X size={14} aria-hidden />
                </button>
            </div>

            <p className="mt-1.5 text-sm text-gray-800">
                <span className="font-semibold">{EVENT_LABEL[alert.event]}</span>
                <span className="font-data-mono text-xs text-gray-500">
                    {" "}· {devicePkLabel(alert.device.pk)} · {alert.code}
                </span>
            </p>

            <div className="mt-2 flex items-center gap-2 text-xs text-gray-500">
                <span className="whitespace-nowrap">
                    Inició <RelativeTime value={alert.started_at} />
                </span>
                <button
                    type="button"
                    onClick={() => {
                        dismiss(alert.id);
                        open(String(alert.id));
                    }}
                    className="ml-auto flex shrink-0 items-center gap-1 whitespace-nowrap rounded px-1.5 py-1 font-semibold text-gray-900 hover:bg-gray-100 focus-visible:outline-2 focus-visible:outline-gray-900"
                >
                    Ver detalle <ArrowRight size={14} aria-hidden />
                </button>
            </div>
        </div>
    );
}
