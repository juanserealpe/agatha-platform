import type { AlertAction } from "@/services/alertManagement";
import { getRowActions, type RowAction } from "./rowActions";

/** Modo con el que se abre el modal de gestión. Va en la URL (?gestion=), por eso en español. */
export type ManagementMode = "clasificar" | "etiquetar" | "lectura";

export const DEFAULT_MANAGEMENT_MODE: ManagementMode = "lectura";

const MODES: readonly ManagementMode[] = ["clasificar", "etiquetar", "lectura"];

export const ROW_ACTION_MODE: Record<RowAction, ManagementMode> = {
    classify: "clasificar",
    label: "etiquetar",
    view: "lectura",
};

/** Modo al abrir una alerta con un solo clic: su gestión pendiente (la acción principal) o, si no tiene, lectura. */
export function primaryManagementMode(available: readonly AlertAction[]): ManagementMode {
    const [primary] = getRowActions(available);
    return primary ? ROW_ACTION_MODE[primary] : DEFAULT_MANAGEMENT_MODE;
}

/** Modo desde la URL; un valor desconocido abre en lectura, que nunca escribe nada. */
export function parseManagementMode(raw: string | null): ManagementMode {
    return MODES.find((mode) => mode === raw) ?? DEFAULT_MANAGEMENT_MODE;
}
