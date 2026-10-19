import type { CSSProperties } from "react";
import { ArrowRight, BellRing, X } from "lucide-react";
import { formatPk } from "@/features/sensors";
import { RelativeTime } from "@/shared/components/RelativeTime";
import { cn } from "@/lib/utils";
import { ALERT_TYPE_LABEL, SEVERITY_STYLES, shouldPulse } from "../config/alertDisplay";
import { useAlertDetailParam } from "../hooks/useAlertDetailParam";
import { useNewAlerts } from "../hooks/useNewAlerts";
import { pickHeadline } from "../lib/newAlerts";

/**
 * Aviso de alerta nueva. No se cierra solo: en monitoreo una alerta no debe desaparecer sin que alguien la vea.
 * Si llegan varias, muestra la más grave y cuántas más hay.
 */
export function NewAlertToast() {
    const { unseen, dismiss } = useNewAlerts();
    const { open } = useAlertDetailParam();
    const alert = pickHeadline(unseen);

    if (!alert) return null;

    const severity = SEVERITY_STYLES[alert.severity];
    const others = unseen.length - 1;
    const isUrgent = alert.severity === "red";

    return (
        <div
            role={isUrgent ? "alert" : "status"}
            aria-live={isUrgent ? "assertive" : "polite"}
            style={{ "--glow-color": severity.hex } as CSSProperties}
            className={cn(
                "w-80 max-w-[calc(100%-1.5rem)] rounded-xl border-2 bg-white p-3 shadow-xl",
                severity.borderClass,
                shouldPulse(alert) && "alert-glow"
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
                <span className="font-semibold">{ALERT_TYPE_LABEL[alert.type]}</span>
                <span className="font-data-mono text-xs text-gray-500">
                    {" "}· Disp. {alert.deviceId}
                    {alert.pkMeters !== null && <> · {formatPk(alert.pkMeters)}</>}
                </span>
            </p>

            <div className="mt-2 flex items-center gap-2 text-xs text-gray-500">
                <span className="whitespace-nowrap">
                    Detectada <RelativeTime value={alert.detectedAt} />
                </span>
                <button
                    type="button"
                    onClick={() => {
                        dismiss(alert.id);
                        open(alert.id);
                    }}
                    className="ml-auto flex shrink-0 items-center gap-1 whitespace-nowrap rounded px-1.5 py-1 font-semibold text-gray-900 hover:bg-gray-100 focus-visible:outline-2 focus-visible:outline-gray-900"
                >
                    Ver detalle <ArrowRight size={14} aria-hidden />
                </button>
            </div>
        </div>
    );
}
