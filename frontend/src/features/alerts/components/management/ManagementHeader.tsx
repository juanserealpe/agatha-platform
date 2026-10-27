import type { AlertItem } from "@/services/alertManagement";
import { Badge } from "@/components/ui/badge";
import { DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { EVENT_LABEL, IN_PROGRESS_LABEL } from "../../config/alertManagementDisplay";
import { deviceDisplayName } from "../../lib/deviceLabel";
import { LevelBadge } from "../badges/LevelBadge";

interface ManagementHeaderProps {
    alert: Pick<AlertItem, "code" | "device" | "event" | "in_progress" | "current_level">;
}

/** ID de la alerta, dispositivo, PK, estado (en curso/finalizada) y nivel actual. */
export function ManagementHeader({ alert }: ManagementHeaderProps) {
    return (
        <DialogHeader className="gap-1.5 pr-8">
            <div className="flex flex-wrap items-center gap-2">
                <DialogTitle className="font-data-mono text-base font-bold text-gray-900">Gestión de alerta {alert.code}</DialogTitle>
                <Badge variant="outline" className="gap-1.5 font-semibold">
                    {alert.in_progress && <span className="size-1.5 rounded-full bg-gray-900 motion-safe:animate-pulse" aria-hidden />}
                    {alert.in_progress ? IN_PROGRESS_LABEL : "Finalizada"}
                </Badge>
                {alert.current_level && (
                    <span className="flex items-center gap-1 text-xs text-gray-500">
                        Nivel actual <LevelBadge level={alert.current_level} />
                    </span>
                )}
            </div>
            <DialogDescription className="font-data-mono text-xs text-gray-500">
                {deviceDisplayName(alert.device.pk)} · Dispositivo {alert.device.id} · {EVENT_LABEL[alert.event]}
            </DialogDescription>
        </DialogHeader>
    );
}
