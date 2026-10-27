import { useId, useState, type FormEvent } from "react";
import type { LucideIcon } from "lucide-react";
import { CircleCheck, CircleX, Loader2, Lock } from "lucide-react";
import type { AlertDetail, Classification, ManagementResult } from "@/services/alertManagement";
import { Button } from "@/components/ui/button";
import { DialogFooter } from "@/components/ui/dialog";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { CLASSIFICATION_LABEL } from "../../config/alertManagementDisplay";
import { useClassifyAlert } from "../../hooks/useAlertManagementMutations";
import { allowedClassifications, validateClassificationForm } from "../../lib/classificationForm";
import { ManagementNotice } from "./ManagementNotice";
import { StepHeading } from "./StepHeading";

// Texto fijo de CA-2.2.6: ante un error la alerta sigue sin clasificar y se conserva la opción elegida.
const SAVE_ERROR = "No fue posible guardar la clasificación. Intente nuevamente.";

// Las observaciones se registran en el paso de etiqueta: la clasificación solo decide real o falsa.
const NO_OBSERVATION = "";

const OPTIONS: Record<Classification, { label: string; description: string; Icon: LucideIcon }> = {
    CONFIRMADA: { label: "Confirmar alerta real", description: "El evento ocurrió o se verificó en campo.", Icon: CircleCheck },
    FALSA_ALERTA: { label: "Falsa alerta", description: "No corresponde a un evento real en el ducto.", Icon: CircleX },
};

interface ClassificationStepProps {
    detail: Pick<AlertDetail, "id" | "available_actions">;
    onSaved: (result: ManagementResult) => void;
    onCancel: () => void;
}

/**
 * Paso 1 (obligatorio): clasificación. Es irreversible, así que no hay opción preseleccionada y se pide
 * una confirmación explícita antes de enviarla (HU-2.2).
 */
export function ClassificationStep({ detail, onSaved, onCancel }: ClassificationStepProps) {
    const titleId = useId();
    const [classification, setClassification] = useState<Classification | null>(null);
    const [confirming, setConfirming] = useState(false);
    const classify = useClassifyAlert(detail.id);

    const allowed = allowedClassifications(detail.available_actions);
    const errors = validateClassificationForm({ classification, observation: NO_OBSERVATION }, allowed);

    function handleSubmit(event: FormEvent<HTMLFormElement>) {
        event.preventDefault();
        if (classification === null || errors.length > 0 || classify.isPending) return;
        if (!confirming) {
            setConfirming(true);
            return;
        }
        classify.mutate({ classification, observation: NO_OBSERVATION }, { onSuccess: onSaved });
    }

    return (
        <form onSubmit={handleSubmit} noValidate aria-labelledby={titleId} className="flex flex-col gap-4">
            <StepHeading id={titleId} step={1} title="Clasificación de la alerta" aside="Obligatoria" />

            {/* Mientras se confirma, la opción queda fija: se confirma exactamente lo que se ve. */}
            <fieldset disabled={classify.isPending || confirming}>
                <RadioGroup
                    aria-labelledby={titleId}
                    aria-required
                    value={classification}
                    onValueChange={(value) => setClassification(allowed.find((option) => option === value) ?? null)}
                    className="grid-cols-1 sm:grid-cols-2"
                >
                    {allowed.map((option) => {
                        const { label, description, Icon } = OPTIONS[option];
                        return (
                            <label
                                key={option}
                                className="flex cursor-pointer items-start gap-3 rounded-lg border border-border px-3 py-3 transition-colors hover:bg-gray-50 has-disabled:cursor-default has-data-checked:border-gray-900 has-data-checked:bg-gray-900 has-data-checked:text-white has-data-checked:hover:bg-gray-800"
                            >
                                <RadioGroupItem value={option} className="mt-0.5 data-checked:border-white" />
                                <span className="flex flex-col gap-0.5">
                                    <span className="flex items-center gap-1.5 text-sm font-semibold">
                                        <Icon className="size-4" aria-hidden />
                                        {label}
                                    </span>
                                    <span className="text-xs opacity-80">{description}</span>
                                </span>
                            </label>
                        );
                    })}
                </RadioGroup>
            </fieldset>

            {classify.isError && <ManagementNotice tone="error">{SAVE_ERROR}</ManagementNotice>}

            {confirming && classification !== null ? (
                <DialogFooter className="-mx-5 -mb-5 items-center px-5 sm:justify-between">
                    <p role="status" className="flex items-start gap-1.5 text-sm text-gray-800">
                        <Lock className="mt-0.5 size-4 shrink-0" aria-hidden />
                        <span>
                            ¿Confirma que la alerta es <strong>{CLASSIFICATION_LABEL[classification]}</strong>? Después no se podrá
                            modificar.
                        </span>
                    </p>
                    <div className="flex flex-col-reverse gap-2 sm:flex-row">
                        <Button type="button" variant="outline" onClick={() => setConfirming(false)} disabled={classify.isPending}>
                            Volver
                        </Button>
                        <Button type="submit" autoFocus disabled={classify.isPending}>
                            {classify.isPending && <Loader2 className="animate-spin" aria-hidden />}
                            {classify.isPending ? "Guardando…" : "Sí, clasificar"}
                        </Button>
                    </div>
                </DialogFooter>
            ) : (
                <DialogFooter className="-mx-5 -mb-5 items-center px-5 sm:justify-between">
                    <p className="flex items-center gap-1.5 text-xs text-gray-600">
                        <Lock className="size-3.5" aria-hidden />
                        Esta clasificación no se puede modificar.
                    </p>
                    <div className="flex flex-col-reverse gap-2 sm:flex-row">
                        <Button type="button" variant="outline" onClick={onCancel}>
                            Ahora no
                        </Button>
                        <Button type="submit" disabled={errors.length > 0}>
                            Guardar clasificación
                        </Button>
                    </div>
                </DialogFooter>
            )}
        </form>
    );
}
