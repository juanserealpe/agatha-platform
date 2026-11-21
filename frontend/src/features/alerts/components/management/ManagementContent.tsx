import { useId, useMemo, useState } from "react";
import type { AlertDetail, ManagementResult } from "@/services/alertManagement";
import { Button } from "@/components/ui/button";
import { DialogFooter } from "@/components/ui/dialog";
import { ALERT_DURATION_LABEL } from "../../config/alertExplanations";
import { useAlertGroup, useOpenGroupMember } from "../../hooks/useAlertGroup";
import { useTimelineNavigation } from "../../hooks/useTimelineNavigation";
import { allowedClassifications } from "../../lib/classificationForm";
import type { ManagementMode } from "../../lib/managementMode";
import { describeExisting } from "../../lib/managementOutcome";
import { compactStripHeightPx, type StripIndicatorInput } from "../../lib/timeStrip";
import { AlertExplanation } from "./AlertExplanation";
import { ClassificationStep } from "./ClassificationStep";
import { EventTimeline } from "./EventTimeline";
import { LabelStep } from "./LabelStep";
import { ManagementHeader } from "./ManagementHeader";
import { ManagementNotice, type NoticeTone } from "./ManagementNotice";
import { ManagementSummary } from "./ManagementSummary";
import { ObservationList } from "./ObservationList";
import { StepHeading } from "./StepHeading";
import { TimeStrip } from "./TimeStrip";

// Aire entre la franja compacta y el título de la fase a la que se navega, para que se lea dónde se está.
const PHASE_SCROLL_GAP_PX = 12;

interface Notice {
    tone: NoticeTone;
    text: string;
}

interface ManagementContentProps {
    detail: AlertDetail;
    initialMode: ManagementMode;
    onClose: () => void;
}

function resultNotice(result: ManagementResult, savedText: string): Notice {
    // 409: otro usuario gestionó primero; prevalece lo suyo y lo enviado queda como observación.
    if (result.outcome === "SAVED_AS_CONFLICT_OBSERVATION") {
        return { tone: "info", text: `${describeExisting(result.existing)} Su registro quedó como observación.` };
    }
    return { tone: "success", text: savedText };
}

/**
 * Coordina los pasos del modal sobre el detalle ya cargado: 1) clasificación (obligatoria) y 2) etiqueta
 * (opcional). Qué paso se puede mostrar lo decide `available_actions`, nunca el estado de la alerta.
 */
export function ManagementContent({ detail, initialMode, onClose }: ManagementContentProps) {
    const [mode, setMode] = useState<ManagementMode>(initialMode);
    const [notice, setNotice] = useState<Notice | null>(null);
    // Recién clasificada en este modal: se ofrece "Agregar etiqueta" / "Ahora no" (CA-2.2.3).
    const [justClassified, setJustClassified] = useState(false);
    const observationsTitleId = useId();
    const endedAt = detail.in_progress ? null : detail.ended_at;
    // Vista combinada: una barra por alerta del punto que se cruza con la abierta (con una sola, como antes).
    const group = useAlertGroup(detail);
    const { openMember, prefetchMember } = useOpenGroupMember(group);
    const combined = group.length > 1;
    const stripIndicators = useMemo<StripIndicatorInput[]>(
        () =>
            group.map((member) => ({
                id: member.id,
                event: member.event,
                levels: member.levels,
                endedAt: member.state === "ACTIVA" ? null : member.ended_at,
                code: combined ? member.code : undefined,
                current: member.id === detail.id,
            })),
        [group, combined, detail.id]
    );
    const navigation = useTimelineNavigation();

    const canClassify = allowedClassifications(detail.available_actions).length > 0;
    const canLabel = detail.available_actions.includes("ADD_LABEL") && detail.classification !== null;

    function goTo(next: ManagementMode) {
        setNotice(null);
        setMode(next);
    }

    function handleClassified(result: ManagementResult) {
        setNotice(resultNotice(result, "Clasificación guardada."));
        setJustClassified(true);
        setMode("lectura");
    }

    function handleLabeled(result: ManagementResult) {
        setNotice(resultNotice(result, "Etiqueta guardada."));
        setJustClassified(false);
        setMode("lectura");
    }

    // Un modo que el backend ya no permite (p. ej. se gestionó en otra pestaña) cae al resumen de lectura.
    let body;
    let inStep = true;
    if (mode === "clasificar" && canClassify) {
        body = <ClassificationStep detail={detail} onSaved={handleClassified} onCancel={onClose} />;
    } else if (mode === "etiquetar" && canLabel && detail.classification !== null) {
        body = (
            <LabelStep
                detail={{ ...detail, classification: detail.classification }}
                stepNumber={2}
                onSaved={handleLabeled}
                onCancel={onClose}
            />
        );
    } else {
        inStep = false;
        body = (
            <>
                <ManagementSummary detail={detail} />
                <section aria-labelledby={observationsTitleId} className="flex flex-col gap-2">
                    <StepHeading id={observationsTitleId} title="Observaciones" />
                    <ObservationList observations={detail.observations} />
                </section>
                {justClassified && canLabel ? (
                    <DialogFooter className="-mx-5 -mb-5 items-center px-5 sm:justify-between">
                        <p className="text-sm text-gray-700">¿Desea agregar ahora la causa del evento? Es opcional.</p>
                        <div className="flex flex-col-reverse gap-2 sm:flex-row">
                            <Button variant="outline" onClick={onClose}>
                                Ahora no
                            </Button>
                            <Button onClick={() => goTo("etiquetar")}>Agregar etiqueta</Button>
                        </div>
                    </DialogFooter>
                ) : (
                    <DialogFooter className="-mx-5 -mb-5 px-5">
                        <Button variant="outline" onClick={onClose}>
                            Cerrar
                        </Button>
                        {canClassify && <Button onClick={() => goTo("clasificar")}>Clasificar</Button>}
                        {canLabel && <Button onClick={() => goTo("etiquetar")}>Agregar etiqueta</Button>}
                    </DialogFooter>
                )}
            </>
        );
    }

    return (
        <>
            <ManagementHeader alert={detail} group={group} />
            {/* -mx-5/px-5 y -top-5: al quedar fija se pega al borde del modal y cubre su relleno (p-5). Sin -top-5
                Chrome la pega a 20 px del borde, el marcador de useIsStuck nunca sale de la vista y no se compacta. */}
            <TimeStrip
                indicators={stripIndicators}
                selected={navigation.selected}
                onSelectPhase={navigation.navigateToPhase}
                onOpenAlert={openMember}
                onPrefetchAlert={prefetchMember}
                hoverRange={navigation.hoverRange}
                className="-top-5 -mx-5 px-5"
            />
            {notice && <ManagementNotice tone={notice.tone}>{notice.text}</ManagementNotice>}
            {/* Contexto del evento en todos los modos: para clasificar hay que entender qué pasó. En los pasos
                la explicación va plegada para que el formulario quede a la vista. */}
            <AlertExplanation detail={detail} collapsed={inStep} />
            <EventTimeline
                levels={detail.levels}
                endedAt={endedAt}
                pendingClassification={detail.management_status === "SIN_CLASIFICAR"}
                controls={navigation}
                scrollMarginTopPx={compactStripHeightPx(stripIndicators.length) + PHASE_SCROLL_GAP_PX}
                durationLabel={combined ? ALERT_DURATION_LABEL : undefined}
            />
            {body}
        </>
    );
}
