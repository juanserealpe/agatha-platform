import { memo } from "react";
import { BellRing } from "lucide-react";
import type { AlertId, AlertItem } from "@/services/alertManagement";
import { TableCell, TableRow } from "@/components/ui/table";
import { formatColombiaDateTime, toIsoString } from "@/shared/lib/formatDate";
import { cn } from "@/lib/utils";
import {
    CLASSIFICATION_LABEL,
    EMPTY_CELL,
    EVENT_LABEL,
    IN_PROGRESS_LABEL,
    labelDisplayText,
} from "../../config/alertManagementDisplay";
import { devicePkLabel } from "../../lib/deviceLabel";
import type { RowAction } from "../../lib/rowActions";
import { LevelBadge } from "../badges/LevelBadge";
import { ManagementStatusBadge } from "../badges/ManagementStatusBadge";
import { AlertRowActions } from "./AlertRowActions";

interface AlertHistoryRowProps {
    item: AlertItem;
    onAction: (id: AlertId, action: RowAction) => void;
}

const MUTED = "text-gray-400";
// Celdas de dos líneas: dato principal arriba y dato relacionado abajo, para que la tabla quepa en pantalla.
const STACK = "flex flex-col items-start gap-1";
const SECONDARY = "text-xs text-gray-500";

function DateTime({ value, className }: { value: string; className?: string }) {
    return (
        <time dateTime={toIsoString(value)} className={cn("font-data-mono text-xs", className)}>
            {formatColombiaDateTime(value)}
        </time>
    );
}

/** Fila del historial. Memoizada: al gestionar una alerta solo se vuelve a pintar su fila. */
export const AlertHistoryRow = memo(function AlertHistoryRow({ item, onAction }: AlertHistoryRowProps) {
    const label = labelDisplayText(item.label, item.label_other_text);

    return (
        <TableRow className={cn(item.blinking && "bg-red-50/60 hover:bg-red-50")}>
            {/* Rojo sin clasificar: borde, ícono y texto, no solo el color de fondo. */}
            <TableCell className={cn("border-l-4 border-l-transparent px-3", item.blinking && "border-l-alerta-roja")}>
                <span className="flex items-center gap-1.5 font-data-mono text-xs font-bold text-gray-900">
                    {item.blinking && (
                        <BellRing className="size-3.5 text-red-600 motion-safe:animate-pulse" aria-hidden>
                            <title>Rojo sin clasificar: requiere atención</title>
                        </BellRing>
                    )}
                    {item.code}
                </span>
                {item.blinking && <span className="sr-only">Requiere atención: rojo sin clasificar</span>}
            </TableCell>

            <TableCell className="px-3">
                <div className={STACK}>
                    <span className="font-data-mono text-xs font-semibold text-gray-900" title={item.device.pk}>
                        {devicePkLabel(item.device.pk)}
                    </span>
                    <span className={cn("font-data-mono", SECONDARY)}>ID {item.device.id}</span>
                </div>
            </TableCell>

            <TableCell className="px-3">
                <div className={STACK}>
                    <span>
                        <span className="sr-only">Inicio: </span>
                        <DateTime value={item.started_at} className="text-gray-900" />
                    </span>
                    <span>
                        <span className="sr-only">Fin: </span>
                        {item.in_progress ? (
                            <span className="inline-flex items-center gap-1.5 text-xs font-semibold text-gray-900">
                                <span className="size-1.5 rounded-full bg-gray-900 motion-safe:animate-pulse" aria-hidden />
                                {IN_PROGRESS_LABEL}
                            </span>
                        ) : item.ended_at ? (
                            <DateTime value={item.ended_at} className="text-gray-500" />
                        ) : (
                            <span className={MUTED}>{EMPTY_CELL}</span>
                        )}
                    </span>
                </div>
            </TableCell>

            <TableCell className="px-3">
                <LevelBadge level={item.current_level} emptyLabel="Finalizada, sin nivel actual" />
            </TableCell>

            <TableCell className="px-3">
                <LevelBadge level={item.max_level} />
            </TableCell>

            <TableCell className="px-3 text-gray-900">{EVENT_LABEL[item.event]}</TableCell>

            <TableCell className="px-3">
                <div className={STACK}>
                    <ManagementStatusBadge status={item.management_status} />
                    {item.classification && (
                        <span className={SECONDARY}>
                            <span className="sr-only">Clasificación: </span>
                            {CLASSIFICATION_LABEL[item.classification]}
                        </span>
                    )}
                </div>
            </TableCell>

            <TableCell className="max-w-48 min-w-32 px-3 text-xs whitespace-normal text-gray-700">
                {label ?? <span className={MUTED}>{EMPTY_CELL}</span>}
            </TableCell>

            <TableCell className="px-3">
                <AlertRowActions item={item} onAction={onAction} />
            </TableCell>
        </TableRow>
    );
});
