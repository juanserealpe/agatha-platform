import { Eye } from "lucide-react";
import type { AlertId, AlertItem } from "@/services/alertManagement";
import { Button } from "@/components/ui/button";
import { EMPTY_CELL, ROW_ACTION_LABEL } from "../../config/alertManagementDisplay";
import { getRowActions, type RowAction } from "../../lib/rowActions";

interface AlertRowActionsProps {
    item: Pick<AlertItem, "id" | "code" | "available_actions">;
    onAction: (id: AlertId, action: RowAction) => void;
    /** El usuario se acerca a un botón: se precarga el detalle para que el modal abra sin espera. */
    onIntent: (id: AlertId) => void;
}

/**
 * Botones de la fila según `available_actions`. Con una acción pendiente, esa es el botón principal y
 * "Ver detalle" se reduce a un ícono a su lado; sin pendientes, "Ver detalle" va como botón con texto.
 */
export function AlertRowActions({ item, onAction, onIntent }: AlertRowActionsProps) {
    const actions = getRowActions(item.available_actions);
    if (actions.length === 0) return <span className="text-gray-400">{EMPTY_CELL}</span>;

    const hasPendingAction = actions.some((action) => action !== "view");
    const handlers = (action: RowAction) => ({
        // El id es siempre el de la ALERTA (items[].id), nunca el del dispositivo.
        onClick: () => onAction(item.id, action),
        onPointerEnter: () => onIntent(item.id),
        onFocus: () => onIntent(item.id),
        "aria-label": `${ROW_ACTION_LABEL[action]} alerta ${item.code}`,
    });

    return (
        <div className="flex items-center gap-1.5">
            {actions.map((action) =>
                action === "view" && hasPendingAction ? (
                    <Button key={action} size="icon-sm" variant="outline" title={ROW_ACTION_LABEL[action]} {...handlers(action)}>
                        <Eye aria-hidden />
                    </Button>
                ) : (
                    <Button key={action} size="sm" variant={action === "view" ? "outline" : "default"} {...handlers(action)}>
                        {ROW_ACTION_LABEL[action]}
                    </Button>
                )
            )}
        </div>
    );
}
