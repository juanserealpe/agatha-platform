import type { AlertAction, Classification } from "@/services/alertManagement";
import { MANAGEMENT_LIMITS } from "@/services/alertManagement";

export interface ClassificationFormValues {
    /** Sin opción preseleccionada: null hasta que el operador elige. */
    classification: Classification | null;
    observation: string;
}

export type ClassificationFormError = "classification" | "observation";

// Cada opción solo se ofrece si el backend la habilita en `available_actions`.
const CLASSIFICATION_ACTION: Record<Classification, AlertAction> = {
    CONFIRMADA: "CONFIRM",
    FALSA_ALERTA: "FALSE_ALERT",
};

export const CLASSIFICATION_ORDER: readonly Classification[] = ["CONFIRMADA", "FALSA_ALERTA"];

export function allowedClassifications(actions: readonly AlertAction[]): Classification[] {
    return CLASSIFICATION_ORDER.filter((classification) => actions.includes(CLASSIFICATION_ACTION[classification]));
}

export function validateClassificationForm(
    values: ClassificationFormValues,
    allowed: readonly Classification[]
): ClassificationFormError[] {
    const errors: ClassificationFormError[] = [];
    if (values.classification === null || !allowed.includes(values.classification)) errors.push("classification");
    if (values.observation.trim().length > MANAGEMENT_LIMITS.observationMaxLength) errors.push("observation");
    return errors;
}
