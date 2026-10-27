import type { AlertAction } from "@/services/alertManagement";

/** Acción de la interfaz. Confirmar y Falsa alerta se agrupan en un solo "Clasificar" que abre el modal. */
export type RowAction = "classify" | "label" | "view";

const ACTION_TO_ROW_ACTION: Record<AlertAction, RowAction> = {
    CONFIRM: "classify",
    FALSE_ALERT: "classify",
    ADD_LABEL: "label",
    VIEW: "view",
};

// La acción pendiente de gestión va primero: es la que se muestra como botón principal.
const ROW_ACTION_ORDER: readonly RowAction[] = ["classify", "label", "view"];

/**
 * Botones de una fila, derivados SOLO de `available_actions` del backend (nunca del estado de la alerta).
 * Acciones desconocidas se ignoran para que un valor nuevo del backend no rompa la tabla.
 */
export function getRowActions(available: readonly AlertAction[]): RowAction[] {
    const actions = new Set(available.map((action) => ACTION_TO_ROW_ACTION[action]).filter(Boolean));
    return ROW_ACTION_ORDER.filter((action) => actions.has(action));
}
