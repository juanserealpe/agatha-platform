import type { AlertLabel } from "@/services/alertManagement";
import { MANAGEMENT_LIMITS, OTHER_LABEL } from "@/services/alertManagement";

export interface LabelFormValues {
    label: AlertLabel | null;
    otherText: string;
    observation: string;
}

export type LabelFormError = "label" | "otherText" | "observation";

/**
 * Errores del formulario de etiqueta. "Guardar etiqueta" se habilita solo sin errores. Valida lo mismo que el
 * backend (etiqueta de la clasificación, texto de OTRO de 1 a 100, observación hasta 500).
 */
export function validateLabelForm(values: LabelFormValues, allowed: readonly AlertLabel[]): LabelFormError[] {
    const errors: LabelFormError[] = [];

    if (values.label === null || !allowed.includes(values.label)) errors.push("label");

    if (values.label === OTHER_LABEL) {
        const length = values.otherText.trim().length;
        if (length === 0 || length > MANAGEMENT_LIMITS.otherTextMaxLength) errors.push("otherText");
    }

    if (values.observation.trim().length > MANAGEMENT_LIMITS.observationMaxLength) errors.push("observation");

    return errors;
}
