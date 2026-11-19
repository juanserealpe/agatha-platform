import { useId, useState, type FormEvent } from "react";
import { Lock } from "lucide-react";
import type { AlertDetail, Classification, ManagementResult } from "@/services/alertManagement";
import { Button } from "@/components/ui/button";
import { DialogFooter } from "@/components/ui/dialog";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { cn } from "@/lib/utils";
import { CLASSIFICATION_OPTIONS } from "../../config/classificationOptions";
import { useClassifyAlert } from "../../hooks/useAlertManagementMutations";
import { allowedClassifications, validateClassificationForm } from "../../lib/classificationForm";
import { ConfirmClassificationDialog } from "./ConfirmClassificationDialog";
import { ManagementNotice } from "./ManagementNotice";
import { StepHeading } from "./StepHeading";

// Texto fijo de CA-2.2.6: ante un error la alerta sigue sin clasificar y se conserva la opción elegida.
const SAVE_ERROR = "No fue posible guardar la clasificación. Intente nuevamente.";

// Las observaciones se registran en el paso de etiqueta: la clasificación solo decide real o falsa.
const NO_OBSERVATION = "";

interface ClassificationStepProps {
    detail: Pick<AlertDetail, "id" | "available_actions">;
    onSaved: (result: ManagementResult) => void;
    onCancel: () => void;
}

/**
 * Paso 1 (obligatorio): clasificación. Es irreversible, así que no hay opción preseleccionada y se pide
 * una confirmación explícita en una ventana aparte antes de enviarla (HU-2.2).
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
        classify.reset();
        setConfirming(true);
    }

    function handleConfirm() {
        if (classification === null) return;
        classify.mutate(
            { classification, observation: NO_OBSERVATION },
            // Ante un error se cierra la confirmación y el aviso queda junto a la opción elegida, que se conserva.
            { onSuccess: onSaved, onError: () => setConfirming(false) }
        );
    }

    function handleConfirmOpenChange(open: boolean) {
        // Mientras se guarda no se puede cerrar: el resultado debe verse en el paso.
        if (!open && classify.isPending) return;
        setConfirming(open);
    }

    return (
        <form onSubmit={handleSubmit} noValidate aria-labelledby={titleId} className="flex flex-col gap-4">
            <StepHeading id={titleId} step={1} title="Clasificación de la alerta" aside="Obligatoria" />

            <fieldset disabled={classify.isPending}>
                <RadioGroup
                    aria-labelledby={titleId}
                    aria-required
                    value={classification}
                    onValueChange={(value) => setClassification(allowed.find((option) => option === value) ?? null)}
                    className="grid-cols-1 sm:grid-cols-2"
                >
                    {allowed.map((option) => {
                        const { label, description, Icon, cardClass, radioClass } = CLASSIFICATION_OPTIONS[option];
                        return (
                            <label
                                key={option}
                                className={cn(
                                    "flex cursor-pointer items-start gap-3 rounded-lg border px-3 py-3 transition-colors has-disabled:cursor-default",
                                    cardClass
                                )}
                            >
                                <RadioGroupItem value={option} className={cn("mt-0.5 bg-white", radioClass)} />
                                <span className="flex flex-col gap-0.5">
                                    <span className="flex items-center gap-1.5 text-sm font-semibold">
                                        <Icon className="size-4" aria-hidden />
                                        {label}
                                    </span>
                                    <span className="text-xs opacity-90">{description}</span>
                                </span>
                            </label>
                        );
                    })}
                </RadioGroup>
            </fieldset>

            {classify.isError && <ManagementNotice tone="error">{SAVE_ERROR}</ManagementNotice>}

            <DialogFooter className="-mx-5 -mb-5 items-center px-5 sm:justify-between">
                <p className="flex items-center gap-1.5 text-xs text-gray-600">
                    <Lock className="size-3.5" aria-hidden />
                    Esta clasificación no se puede modificar.
                </p>
                <div className="flex flex-col-reverse gap-2 sm:flex-row">
                    <Button type="button" variant="outline" onClick={onCancel}>
                        Ahora no
                    </Button>
                    <Button type="submit" disabled={errors.length > 0 || classify.isPending}>
                        Guardar clasificación
                    </Button>
                </div>
            </DialogFooter>

            {classification !== null && (
                <ConfirmClassificationDialog
                    open={confirming}
                    onOpenChange={handleConfirmOpenChange}
                    classification={classification}
                    pending={classify.isPending}
                    onConfirm={handleConfirm}
                />
            )}
        </form>
    );
}
