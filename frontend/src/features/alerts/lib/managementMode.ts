import type { RowAction } from "./rowActions";

/** Modo con el que se abre el modal de gestión. Va en la URL (?gestion=), por eso en español. */
export type ManagementMode = "clasificar" | "etiquetar" | "lectura";

export const DEFAULT_MANAGEMENT_MODE: ManagementMode = "lectura";

const MODES: readonly ManagementMode[] = ["clasificar", "etiquetar", "lectura"];

export const ROW_ACTION_MODE: Record<RowAction, ManagementMode> = {
    classify: "clasificar",
    label: "etiquetar",
    view: "lectura",
};

/** Modo desde la URL; un valor desconocido abre en lectura, que nunca escribe nada. */
export function parseManagementMode(raw: string | null): ManagementMode {
    return MODES.find((mode) => mode === raw) ?? DEFAULT_MANAGEMENT_MODE;
}
