import { useId } from "react";
import type { LevelSegment } from "@/services/alertManagement";
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";
import { Badge } from "@/components/ui/badge";
import { useNow } from "@/shared/hooks/useNow";
import { formatColombiaDateTime, toIsoString } from "@/shared/lib/formatDate";
import { cn } from "@/lib/utils";
import { levelStyle } from "../../config/alertManagementDisplay";
import { GENERAL_DURATION_LABEL, OPENED_BY_LABEL } from "../../config/alertExplanations";
import type { TimelineNavigation } from "../../hooks/useTimelineNavigation";
import { formatDuration, segmentDurationMs, totalDurationMs } from "../../lib/timeline";
import type { HoverRange } from "../../lib/timeStrip";
import { LevelBadge } from "../badges/LevelBadge";
import { PhaseMessage } from "./PhaseMessage";
import { StepHeading } from "./StepHeading";

function DateTime({ value }: { value: string }) {
    return <time dateTime={toIsoString(value)}>{formatColombiaDateTime(value)}</time>;
}

const MESSAGES_ITEM = "messages";

/** Parte de la navegación que usa la línea de tiempo; la franja comparte el mismo estado. */
export type TimelinePhaseControls = Pick<
    TimelineNavigation,
    "expandedSeqs" | "togglePhase" | "arrivedSeq" | "navigating" | "setHoverRange" | "phaseElementId"
>;

function phaseRange(segment: LevelSegment): HoverRange {
    return {
        startMs: new Date(segment.started_at).getTime(),
        endMs: segment.ended_at ? new Date(segment.ended_at).getTime() : null,
    };
}

interface SegmentItemProps {
    segment: LevelSegment;
    isLast: boolean;
    now: number;
    controls: TimelinePhaseControls;
    scrollMarginTopPx: number;
}

function SegmentItem({ segment, isLast, now, controls, scrollMarginTopPx }: SegmentItemProps) {
    const style = levelStyle(segment.level);
    const isOpen = segment.ended_at === null;
    const messageCount = segment.messages.length;
    const expanded = controls.expandedSeqs.has(segment.seq);
    const range = phaseRange(segment);

    return (
        <li
            id={controls.phaseElementId(segment.seq)}
            // Destino de la navegación desde la franja: recibe el foco (lector de pantalla) y queda bajo la
            // franja fija, no detrás de ella (§5.3, paso 3).
            tabIndex={-1}
            style={{ scrollMarginTop: scrollMarginTopPx }}
            className={cn(
                "-mx-1.5 flex gap-3 rounded-md px-1.5 pt-1 ring-2 ring-transparent outline-none transition-shadow duration-300",
                controls.arrivedSeq === segment.seq && "ring-gray-900/50"
            )}
            onMouseEnter={() => controls.setHoverRange(range)}
            onMouseLeave={() => controls.setHoverRange(null)}
        >
            <span className="flex flex-col items-center pt-1" aria-hidden>
                <span className={cn("size-3 shrink-0 rounded-full ring-4", style.dotClass, isOpen ? "ring-gray-200" : "ring-white")} />
                {!isLast && <span className="mt-1 w-px flex-1 bg-gray-200" />}
            </span>
            <div className={cn("min-w-0 flex-1", !isLast && "pb-4")}>
                <div className="flex flex-wrap items-center gap-2">
                    <LevelBadge level={segment.level} />
                    <span className="text-xs text-gray-500">{OPENED_BY_LABEL[segment.opened_by] ?? segment.opened_by}</span>
                    {isOpen && <span className="text-xs font-semibold text-gray-900">Nivel actual</span>}
                </div>
                <p className="mt-1 font-data-mono text-xs text-gray-600">
                    <DateTime value={segment.started_at} /> → {segment.ended_at ? <DateTime value={segment.ended_at} /> : "en curso"}
                    {" · "}
                    {formatDuration(segmentDurationMs(segment, now))}
                </p>
                {messageCount > 0 && (
                    // Controlado: las fases solo se pliegan o despliegan por clic o al navegar desde la franja,
                    // nunca por hacer scroll (§5.3).
                    <Accordion
                        value={expanded ? [MESSAGES_ITEM] : []}
                        onValueChange={(value) => controls.togglePhase(segment.seq, value.includes(MESSAGES_ITEM))}
                    >
                        <AccordionItem value={MESSAGES_ITEM} className="border-none">
                            <AccordionTrigger className="py-1.5 text-xs text-gray-600">
                                {messageCount === 1 ? "Ver 1 mensaje" : `Ver ${messageCount} mensajes`}
                            </AccordionTrigger>
                            <AccordionContent>
                                <ul className="flex flex-col gap-1.5">
                                    {segment.messages.map((message) => (
                                        <PhaseMessage
                                            key={message.id}
                                            message={message}
                                            phaseLevel={segment.level}
                                            onHoverChange={(hovering) => {
                                                const at = new Date(message.at).getTime();
                                                controls.setHoverRange(hovering ? { startMs: at, endMs: at } : range);
                                            }}
                                        />
                                    ))}
                                </ul>
                            </AccordionContent>
                        </AccordionItem>
                    </Accordion>
                )}
            </div>
        </li>
    );
}

// Cierre del evento: la medición volvió a lo normal. Usa el verde de "normal" del mapa, no un color de severidad.
// La insignia es neutra a propósito: no es un nivel más, así que no debe leerse como "bajó de Rojo a Normal"
// (el backend finaliza por medición normal aunque el rojo siga sin clasificar).
function EndItem({ endedAt, pendingClassification }: { endedAt: string; pendingClassification: boolean }) {
    return (
        <li className="flex gap-3 pt-1">
            <span className="flex flex-col items-center pt-1" aria-hidden>
                <span className="size-3 shrink-0 rounded-full bg-alerta-verde ring-4 ring-white" />
            </span>
            <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-center gap-2">
                    <Badge className="bg-gray-100 font-semibold text-gray-700 ring-1 ring-gray-200">Finalizada</Badge>
                    <span className="text-xs text-gray-500">La medición volvió a lo normal</span>
                    {pendingClassification && (
                        <span className="text-xs font-semibold text-gray-900">Pendiente de clasificación</span>
                    )}
                </div>
                <p className="mt-1 font-data-mono text-xs text-gray-600">
                    <DateTime value={endedAt} />
                </p>
            </div>
        </li>
    );
}

interface EventTimelineProps {
    levels: LevelSegment[];
    /** Fin de la alerta; null mientras sigue en curso. */
    endedAt: string | null;
    /** Finalizó sin clasificar: se avisa en el cierre para que no parezca un caso resuelto. */
    pendingClassification: boolean;
    controls: TimelinePhaseControls;
    /** Alto de la franja fija: la fase a la que se navega queda debajo de ella. */
    scrollMarginTopPx: number;
    /** Rótulo de la duración: "Duración general" con una alerta sola; con varias, la general va en la cabecera. */
    durationLabel?: string;
}

/** Colores por los que pasó la alerta hasta volver a lo normal, con fechas, duración y mensajes (RN-20). */
export function EventTimeline({
    levels,
    endedAt,
    pendingClassification,
    controls,
    scrollMarginTopPx,
    durationLabel = GENERAL_DURATION_LABEL,
}: EventTimelineProps) {
    const titleId = useId();
    const now = useNow();

    return (
        <section aria-labelledby={titleId} className="flex flex-col gap-3">
            <StepHeading
                id={titleId}
                title="Línea de tiempo del evento"
                aside={
                    levels.length > 0 && (
                        <>
                            {durationLabel}: <strong className="font-data-mono text-gray-900">{formatDuration(totalDurationMs(levels, now))}</strong>
                        </>
                    )
                }
            />
            {levels.length === 0 ? (
                <p className="text-sm text-gray-500">Todavía no hay cambios de nivel registrados.</p>
            ) : (
                <ol
                    data-navigating={controls.navigating || undefined}
                    // Al navegar desde la franja las fases que se pliegan lo hacen sin animación (ver useTimelineNavigation);
                    // la de destino sí anima: se abre bajo su título y no lo mueve.
                    className="flex flex-col rounded-lg border border-border px-3 pt-2 pb-3 data-navigating:**:data-[slot=accordion-content]:data-closed:animate-none"
                >
                    {levels.map((segment, index) => (
                        <SegmentItem
                            key={segment.seq}
                            segment={segment}
                            isLast={index === levels.length - 1 && endedAt === null}
                            now={now}
                            controls={controls}
                            scrollMarginTopPx={scrollMarginTopPx}
                        />
                    ))}
                    {endedAt && <EndItem endedAt={endedAt} pendingClassification={pendingClassification} />}
                </ol>
            )}
        </section>
    );
}
