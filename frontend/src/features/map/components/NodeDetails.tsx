import { Activity, ArrowRight } from "lucide-react";
import { ALERT_STATUS_LABEL, ALERT_TYPE_LABEL, SEVERITY_STYLES } from "@/features/alerts";
import { formatPk } from "@/features/sensors";
import { RelativeTime } from "@/shared/components/RelativeTime";
import { cn } from "@/lib/utils";
import { NODE_TONE_STYLES, WARNING_HEX } from "../config/mapStyles";
import type { MapNode } from "../hooks/usePilotNodes";

function StatusLine({ node }: { node: MapNode }) {
    const tone = NODE_TONE_STYLES[node.state.tone];
    return (
        <p className="flex items-center gap-1.5">
            <span className="h-2.5 w-2.5 shrink-0 rounded-full" style={{ background: tone.hex }} aria-hidden />
            {tone.label}
            {node.state.hasWarning && (
                <span className="flex items-center gap-1 text-gray-500">
                    · <span className="h-2 w-2 rounded-full" style={{ background: WARNING_HEX }} aria-hidden /> sin comunicación
                </span>
            )}
        </p>
    );
}

function LastCommunication({ node }: { node: MapNode }) {
    if (!node.device) return <span className="text-gray-500">Sin dispositivo instalado</span>;
    return (
        <span>
            Última comunicación: <RelativeTime value={node.device.lastConnection} className="font-medium" />
        </span>
    );
}

/** Información rápida al pasar el cursor (solo lectura). */
export function NodeTooltipContent({ node }: { node: MapNode }) {
    return (
        <div className="flex flex-col gap-1 font-sans text-xs text-gray-700">
            <p className="font-semibold text-gray-900">
                {node.label} · <span className="font-data-mono">{formatPk(node.pkMeters)}</span>
            </p>
            <StatusLine node={node} />
            <LastCommunication node={node} />
        </div>
    );
}

interface NodePopupContentProps {
    node: MapNode;
    onOpenAlert: (alertId: string) => void;
    onShowTelemetry?: (deviceId: number) => void;
}

const ACTION_CLASS =
    "flex w-full items-center justify-center gap-1.5 rounded-md px-3 py-2 text-xs font-semibold transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-gray-900";

/** Detalle del punto al hacer clic, con las acciones disponibles. */
export function NodePopupContent({ node, onOpenAlert, onShowTelemetry }: NodePopupContentProps) {
    const { activeAlert, device } = node;

    return (
        <div className="flex w-60 flex-col gap-3 font-sans text-xs text-gray-700">
            <div>
                <p className="text-sm font-bold text-gray-900">{node.label}</p>
                <p className="font-data-mono text-gray-500">{formatPk(node.pkMeters)}</p>
            </div>

            {activeAlert && (
                <div className="flex flex-wrap items-center gap-1.5">
                    <span className={cn("rounded-full px-2 py-0.5 font-semibold", SEVERITY_STYLES[activeAlert.severity].badgeClass)}>
                        {SEVERITY_STYLES[activeAlert.severity].label}
                    </span>
                    <span className="font-semibold text-gray-900">{ALERT_TYPE_LABEL[activeAlert.type]}</span>
                    <span className="text-gray-500">· {ALERT_STATUS_LABEL[activeAlert.status]}</span>
                </div>
            )}

            <div className="flex flex-col gap-1">
                <StatusLine node={node} />
                {device && <p className="truncate">Dispositivo: <span className="font-medium">{device.name}</span></p>}
                <LastCommunication node={node} />
            </div>

            {(activeAlert || (device && onShowTelemetry)) && (
                <div className="flex flex-col gap-1.5">
                    {activeAlert && (
                        <button
                            type="button"
                            onClick={() => onOpenAlert(activeAlert.id)}
                            className={cn(ACTION_CLASS, "bg-gray-900 text-white hover:bg-gray-700")}
                        >
                            Ver detalle de la alerta <ArrowRight size={14} aria-hidden />
                        </button>
                    )}
                    {device && onShowTelemetry && (
                        <button
                            type="button"
                            onClick={() => onShowTelemetry(device.id)}
                            className={cn(ACTION_CLASS, "border border-gray-300 text-gray-700 hover:bg-gray-50")}
                        >
                            <Activity size={14} aria-hidden /> Ver telemetría
                        </button>
                    )}
                </div>
            )}
        </div>
    );
}
