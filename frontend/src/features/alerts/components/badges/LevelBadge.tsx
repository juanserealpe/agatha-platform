import type { AlertLevel } from "@/services/alertManagement";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import { EMPTY_CELL, levelStyle } from "../../config/alertManagementDisplay";

interface LevelBadgeProps {
    /** null cuando la alerta ya no tiene nivel actual (finalizó). */
    level: AlertLevel | null;
    emptyLabel?: string;
}

/** Nivel de criticidad con el color de la escala de la app y su nombre (no depende solo del color). */
export function LevelBadge({ level, emptyLabel = "Sin nivel" }: LevelBadgeProps) {
    if (level === null) {
        return (
            <span className="text-gray-400" title={emptyLabel}>
                {EMPTY_CELL}
                <span className="sr-only">{emptyLabel}</span>
            </span>
        );
    }

    const style = levelStyle(level);
    return (
        <Badge className={cn("gap-1.5 font-semibold", style.badgeClass)}>
            <span className={cn("size-1.5 rounded-full", style.dotClass)} aria-hidden />
            {style.label}
        </Badge>
    );
}
