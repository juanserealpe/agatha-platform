import type { AlertId, AlertItem } from "@/services/alertManagement";
import { Button } from "@/components/ui/button";
import { EMPTY_CELL, ROW_ACTION_LABEL } from "../../config/alertManagementDisplay";
import { getRowActions, type RowAction } from "../../lib/rowActions";

interface AlertRowActionsProps {
    item: Pick<AlertItem, "id" | "code" | "available_actions">;
    onAction: (id: AlertId, action: RowAction) => void;
}

/** Botones de la fila según `available_actions`. La acción pendiente de gestión es la principal. */
export function AlertRowActions({ item, onAction }: AlertRowActionsProps) {
    const actions = getRowActions(item.available_actions);
    if (actions.length === 0) return <span className="text-gray-400">{EMPTY_CELL}</span>;

    return (
        <div className="flex items-center gap-1.5">
            {actions.map((action, index) => (
                <Button
                    key={action}
                    size="sm"
                    variant={index === 0 && action !== "view" ? "default" : "outline"}
                    aria-label={`${ROW_ACTION_LABEL[action]} alerta ${item.code}`}
                    // El id es siempre el de la ALERTA (items[].id), nunca el del dispositivo.
                    onClick={() => onAction(item.id, action)}
                >
                    {ROW_ACTION_LABEL[action]}
                </Button>
            ))}
        </div>
    );
}
