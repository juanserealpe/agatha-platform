import type {
    AlertLabel,
    Classification,
    ClassificationRequest,
    LabelRequest,
    ObservationRequest,
} from "@/services/alertManagement";
import { OTHER_LABEL } from "@/services/alertManagement";

// Desde la web toda escritura lleva este origen (RN-10: aún sin autenticación).
const WEB_ORIGIN = "WEB";

export interface ClassifyInput {
    classification: Classification;
    observation: string;
}

export interface LabelInput {
    label: AlertLabel;
    /** Solo se envía con OTRO. */
    otherText: string;
    observation: string;
}

export interface ObservationInput {
    text: string;
}

/** Texto sin espacios sobrantes, o undefined si queda vacío (así el campo no viaja en el JSON). */
function optionalText(value: string): string | undefined {
    const trimmed = value.trim();
    return trimmed === "" ? undefined : trimmed;
}

// Los cuerpos llevan exactamente los campos del contrato: el backend rechaza campos desconocidos con 400.

export function buildClassificationRequest(input: ClassifyInput, clientRecordId?: string): ClassificationRequest {
    return {
        classification: input.classification,
        observation: optionalText(input.observation),
        origin: WEB_ORIGIN,
        client_record_id: clientRecordId,
    };
}

export function buildLabelRequest(input: LabelInput, clientRecordId?: string): LabelRequest {
    return {
        label: input.label,
        // Con otra etiqueta `other_text` no se envía: el backend responde 400 si llega.
        other_text: input.label === OTHER_LABEL ? input.otherText.trim() : undefined,
        observation: optionalText(input.observation),
        origin: WEB_ORIGIN,
        client_record_id: clientRecordId,
    };
}

export function buildObservationRequest(input: ObservationInput, clientRecordId?: string): ObservationRequest {
    return { text: input.text.trim(), origin: WEB_ORIGIN, client_record_id: clientRecordId };
}
