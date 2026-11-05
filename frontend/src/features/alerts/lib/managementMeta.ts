import type { ManagementOrigin } from "@/services/alertManagement";
import { formatColombiaDateTime } from "@/shared/lib/formatDate";
import { ORIGIN_LABEL } from "../config/alertManagementDisplay";

/** "Web · 24/08/2026 12:05": origen y fecha-hora de una clasificación o etiqueta. */
export function managementMeta(step: { origin: ManagementOrigin; at: string | null }): string {
    const origin = ORIGIN_LABEL[step.origin] ?? step.origin;
    return step.at ? `${origin} · ${formatColombiaDateTime(step.at)}` : origin;
}
