import { useId, useState, type FormEvent } from "react";
import { Loader2, Save } from "lucide-react";
import type { AlertDetail, AlertLabel, Classification, ManagementResult } from "@/services/alertManagement";
import { LABELS_BY_CLASSIFICATION, MANAGEMENT_LIMITS, OTHER_LABEL } from "@/services/alertManagement";
import { isApiError } from "@/services/http";
import { Button } from "@/components/ui/button";
import { DialogFooter } from "@/components/ui/dialog";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { CLASSIFICATION_LABEL, LABEL_TEXT } from "../../config/alertManagementDisplay";
import { ALERT_EVIDENCE_ENABLED } from "../../config/featureFlags";
import { useLabelAlert } from "../../hooks/useAlertManagementMutations";
import { validateLabelForm } from "../../lib/labelForm";
import { CountedTextField } from "./CountedTextField";
import { EvidenceSection } from "./EvidenceSection";
import { ManagementNotice } from "./ManagementNotice";
import { ObservationList } from "./ObservationList";
import { StepHeading } from "./StepHeading";

const SAVE_ERROR = "No fue posible guardar la etiqueta. Intente nuevamente.";
const CONFLICT_STATUS = 409;

interface LabelStepProps {
    detail: Pick<AlertDetail, "id" | "observations"> & { classification: Classification };
    /** El paso 2 lleva número cuando sigue a la clasificación en el mismo modal. */
    stepNumber?: number;
    onSaved: (result: ManagementResult) => void;
    onCancel: () => void;
}

/** Paso 2 (opcional): causa del evento. Solo ofrece las etiquetas de la clasificación de la alerta (HU-2.3). */
export function LabelStep({ detail, stepNumber, onSaved, onCancel }: LabelStepProps) {
    const titleId = useId();
    const [label, setLabel] = useState<AlertLabel | null>(null);
    const [otherText, setOtherText] = useState("");
    const [observation, setObservation] = useState("");
    const labelAlert = useLabelAlert(detail.id);

    const allowed: readonly AlertLabel[] = LABELS_BY_CLASSIFICATION[detail.classification];
    const errors = validateLabelForm({ label, otherText, observation }, allowed);
    const isOther = label === OTHER_LABEL;

    function handleSubmit(event: FormEvent<HTMLFormElement>) {
        event.preventDefault();
        if (label === null || errors.length > 0 || labelAlert.isPending) return;
        // Ante un error se conserva todo lo escrito para poder reintentar (mismo client_record_id).
        labelAlert.mutate({ label, otherText, observation }, { onSuccess: onSaved });
    }

    const saveError = labelAlert.isError
        ? isApiError(labelAlert.error) && labelAlert.error.status === CONFLICT_STATUS
            ? labelAlert.error.message
            : SAVE_ERROR
        : null;

    return (
        <form onSubmit={handleSubmit} noValidate aria-labelledby={titleId} className="flex flex-col gap-4">
            <StepHeading
                id={titleId}
                step={stepNumber}
                title="Causa y etiquetado del evento"
                aside={`Clasificada como ${CLASSIFICATION_LABEL[detail.classification]}`}
            />

            <fieldset disabled={labelAlert.isPending} className="flex flex-col gap-4">
                <RadioGroup
                    aria-labelledby={titleId}
                    aria-required
                    value={label}
                    onValueChange={(value) => setLabel(allowed.find((option) => option === value) ?? null)}
                    className="grid-cols-1 sm:grid-cols-2"
                >
                    {allowed.map((option) => (
                        <label
                            key={option}
                            className="flex cursor-pointer items-center gap-2.5 rounded-lg border border-border px-3 py-2.5 text-sm text-gray-800 transition-colors hover:bg-gray-50 has-data-checked:border-gray-900 has-data-checked:bg-gray-50 has-data-checked:font-medium"
                        >
                            <RadioGroupItem value={option} />
                            {LABEL_TEXT[option]}
                        </label>
                    ))}
                </RadioGroup>

                {/* "Otro" despliega su texto obligatorio; con otra etiqueta se oculta y no se envía. */}
                {isOther && (
                    <CountedTextField
                        label="¿Cuál?"
                        value={otherText}
                        onChange={setOtherText}
                        maxLength={MANAGEMENT_LIMITS.otherTextMaxLength}
                        required
                        invalid={errors.includes("otherText") && otherText.length > 0}
                        placeholder="Describa la causa"
                        autoFocus
                    />
                )}

                <CountedTextField
                    label="Observación del operador (opcional)"
                    value={observation}
                    onChange={setObservation}
                    maxLength={MANAGEMENT_LIMITS.observationMaxLength}
                    multiline
                    placeholder="Detalles de la verificación en campo, cuadrilla, acciones tomadas…"
                />

                {ALERT_EVIDENCE_ENABLED && <EvidenceSection />}
            </fieldset>

            {detail.observations.length > 0 && (
                <div className="flex flex-col gap-2">
                    <p className="font-data-mono text-xs font-bold tracking-wider text-gray-600 uppercase">Observaciones previas</p>
                    <ObservationList observations={detail.observations} />
                </div>
            )}

            {saveError && <ManagementNotice tone="error">{saveError}</ManagementNotice>}

            <DialogFooter className="-mx-5 -mb-5 px-5">
                <Button type="button" variant="outline" onClick={onCancel}>
                    Ahora no
                </Button>
                <Button type="submit" disabled={errors.length > 0 || labelAlert.isPending}>
                    {labelAlert.isPending ? <Loader2 className="animate-spin" aria-hidden /> : <Save aria-hidden />}
                    {labelAlert.isPending ? "Guardando…" : "Guardar etiqueta"}
                </Button>
            </DialogFooter>
        </form>
    );
}
