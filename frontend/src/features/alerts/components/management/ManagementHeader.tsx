import { useMemo } from "react";
import type { AlertItem } from "@/services/alertManagement";
import { Badge } from "@/components/ui/badge";
import { DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { useNow } from "@/shared/hooks/useNow";
import { GENERAL_DURATION_LABEL, GENERAL_LEVEL_LABEL } from "../../config/alertExplanations";
import { EVENT_LABEL, IN_PROGRESS_LABEL } from "../../config/alertManagementDisplay";
import { summarizeGroup, type GroupCandidate } from "../../lib/alertGroup";
import { deviceDisplayName } from "../../lib/deviceLabel";
import { formatDuration } from "../../lib/timeline";
import { LevelBadge } from "../badges/LevelBadge";

const FINISHED_LABEL = "Finalizada";

interface ManagementHeaderProps {
    alert: Pick<AlertItem, "code" | "device" | "event" | "in_progress" | "current_level">;
    /** Alertas del punto en la vista (siempre incluye la abierta). Con más de una, la cabecera muestra el grupo. */
    group: readonly GroupCandidate[];
}

function StateBadge({ inProgress }: { inProgress: boolean }) {
    return (
        <Badge variant="outline" className="gap-1.5 font-semibold">
            {inProgress && <span className="size-1.5 rounded-full bg-gray-900 motion-safe:animate-pulse" aria-hidden />}
            {inProgress ? IN_PROGRESS_LABEL : FINISHED_LABEL}
        </Badge>
    );
}

/**
 * ID de la alerta, dispositivo, PK, estado y nivel. Con una alerta sola: su estado y su nivel actual. Con varias
 * alertas del punto a la vez (vista combinada): estado, nivel y duración GENERALES (D4, D6); el estado de la abierta
 * se aclara solo si difiere del grupo.
 */
export function ManagementHeader({ alert, group }: ManagementHeaderProps) {
    const now = useNow();
    const summary = useMemo(() => (group.length > 1 ? summarizeGroup(group, now) : null), [group, now]);
    const ownStateDiffers = summary !== null && summary.inProgress !== alert.in_progress;

    return (
        <DialogHeader className="gap-1.5 pr-8">
            <div className="flex flex-wrap items-center gap-2">
                <DialogTitle className="font-data-mono text-base font-bold text-gray-900">Gestión de alerta {alert.code}</DialogTitle>
                <StateBadge inProgress={summary?.inProgress ?? alert.in_progress} />
                {summary ? (
                    <span className="flex items-center gap-1 text-xs text-gray-500">
                        {GENERAL_LEVEL_LABEL} <LevelBadge level={summary.level} />
                    </span>
                ) : (
                    alert.current_level && (
                        <span className="flex items-center gap-1 text-xs text-gray-500">
                            Nivel actual <LevelBadge level={alert.current_level} />
                        </span>
                    )
                )}
            </div>
            <DialogDescription className="font-data-mono text-xs text-gray-500">
                {deviceDisplayName(alert.device.pk)} · Dispositivo {alert.device.id} · {EVENT_LABEL[alert.event]}
                {ownStateDiffers && ` (esta alerta: ${alert.in_progress ? IN_PROGRESS_LABEL.toLowerCase() : FINISHED_LABEL.toLowerCase()})`}
            </DialogDescription>
            {summary && (
                <p className="text-xs text-gray-600">
                    {summary.size} alertas del punto se cruzan en el tiempo · {GENERAL_DURATION_LABEL}:{" "}
                    <strong className="font-data-mono text-gray-900">{formatDuration(summary.durationMs)}</strong>
                </p>
            )}
        </DialogHeader>
    );
}
