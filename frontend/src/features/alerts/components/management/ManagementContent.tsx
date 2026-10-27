import { useState } from "react";
import type { AlertDetail, ManagementResult } from "@/services/alertManagement";
import { Button } from "@/components/ui/button";
import { DialogFooter } from "@/components/ui/dialog";
import type { ManagementMode } from "../../lib/managementMode";
import { describeExisting } from "../../lib/managementOutcome";
import { LabelStep } from "./LabelStep";
import { ManagementHeader } from "./ManagementHeader";
import { ManagementNotice, type NoticeTone } from "./ManagementNotice";
import { ManagementSummary } from "./ManagementSummary";

interface Notice {
    tone: NoticeTone;
    text: string;
}

interface ManagementContentProps {
    detail: AlertDetail;
    initialMode: ManagementMode;
    onClose: () => void;
}

function labelResultNotice(result: ManagementResult): Notice {
    if (result.outcome === "SAVED_AS_CONFLICT_OBSERVATION") {
        return { tone: "info", text: `${describeExisting(result.existing)} Su registro quedó como observación.` };
    }
    return { tone: "success", text: "Etiqueta guardada." };
}

/** Coordina los pasos del modal sobre el detalle ya cargado. Las acciones salen de `available_actions`. */
export function ManagementContent({ detail, initialMode, onClose }: ManagementContentProps) {
    const [mode, setMode] = useState<ManagementMode>(initialMode);
    const [notice, setNotice] = useState<Notice | null>(null);

    const canLabel = detail.available_actions.includes("ADD_LABEL") && detail.classification !== null;
    // Un modo que el backend ya no permite (p. ej. la alerta se etiquetó en otra pestaña) cae a lectura.
    const showLabelStep = mode === "etiquetar" && canLabel && detail.classification !== null;

    function handleLabelSaved(result: ManagementResult) {
        setNotice(labelResultNotice(result));
        setMode("lectura");
    }

    return (
        <>
            <ManagementHeader alert={detail} />

            {notice && <ManagementNotice tone={notice.tone}>{notice.text}</ManagementNotice>}

            {showLabelStep && detail.classification !== null ? (
                <LabelStep
                    detail={{ ...detail, classification: detail.classification }}
                    onSaved={handleLabelSaved}
                    onCancel={onClose}
                />
            ) : (
                <>
                    <ManagementSummary detail={detail} />
                    <DialogFooter className="-mx-5 -mb-5 px-5">
                        <Button variant="outline" onClick={onClose}>
                            Cerrar
                        </Button>
                        {canLabel && (
                            <Button
                                onClick={() => {
                                    setNotice(null);
                                    setMode("etiquetar");
                                }}
                            >
                                Agregar etiqueta
                            </Button>
                        )}
                    </DialogFooter>
                </>
            )}
        </>
    );
}
