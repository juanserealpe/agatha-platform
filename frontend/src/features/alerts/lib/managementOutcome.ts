import type { ExistingManagement } from "@/services/alertManagement";
import { CLASSIFICATION_LABEL, labelDisplayText, ORIGIN_LABEL } from "../config/alertManagementDisplay";

/**
 * Aviso cuando otro usuario gestionó primero (409): prevalece lo que ya existía.
 * Ej.: "Ya estaba clasificada como Confirmada desde Web."
 */
export function describeExisting(existing: ExistingManagement | undefined): string {
    if (!existing) return "Otro usuario ya había gestionado esta alerta. Se conserva lo que registró primero.";

    const origin = ORIGIN_LABEL[existing.origin] ?? existing.origin;
    if (existing.label) {
        return `Ya estaba etiquetada como «${labelDisplayText(existing.label, existing.label_other_text)}» desde ${origin}.`;
    }
    if (existing.classification) {
        return `Ya estaba clasificada como ${CLASSIFICATION_LABEL[existing.classification]} desde ${origin}.`;
    }
    return `Otro usuario ya había gestionado esta alerta desde ${origin}. Se conserva lo que registró primero.`;
}
