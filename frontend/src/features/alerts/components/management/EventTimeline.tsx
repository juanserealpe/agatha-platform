import { useId } from "react";
import type { LevelSegment, TimelineMessage } from "@/services/alertManagement";
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";
import { Badge } from "@/components/ui/badge";
import { useNow } from "@/shared/hooks/useNow";
import { formatColombiaDateTime, toIsoString } from "@/shared/lib/formatDate";
import { cn } from "@/lib/utils";
import { levelStyle } from "../../config/alertManagementDisplay";
import { MESSAGE_KIND_LABEL, OPENED_BY_LABEL } from "../../config/alertExplanations";
import { formatDuration, segmentDurationMs, totalDurationMs } from "../../lib/timeline";
import { LevelBadge } from "../badges/LevelBadge";
import { StepHeading } from "./StepHeading";

function DateTime({ value }: { value: string }) {
    return <time dateTime={toIsoString(value)}>{formatColombiaDateTime(value)}</time>;
}

function MessageItem({ message }: { message: TimelineMessage }) {
    // title y body ya vienen redactados por el backend: se muestran como texto, nunca como HTML.
    return (
        <li className="flex flex-col gap-0.5 rounded-md bg-gray-50 px-2.5 py-2">
            <span className="font-data-mono text-xs text-gray-500">
                <DateTime value={message.at} /> · {MESSAGE_KIND_LABEL[message.kind] ?? message.kind}
            </span>
            <span className="text-sm font-medium text-gray-900">{message.title}</span>
            {message.body && <span className="text-sm whitespace-pre-line text-gray-700">{message.body}</span>}
        </li>
    );
}

function SegmentItem({ segment, isLast, now }: { segment: LevelSegment; isLast: boolean; now: number }) {
    const style = levelStyle(segment.level);
    const isOpen = segment.ended_at === null;
    const messageCount = segment.messages.length;

    return (
        <li className="flex gap-3">
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
                    <Accordion>
                        <AccordionItem value="messages" className="border-none">
                            <AccordionTrigger className="py-1.5 text-xs text-gray-600">
                                {messageCount === 1 ? "Ver 1 mensaje" : `Ver ${messageCount} mensajes`}
                            </AccordionTrigger>
                            <AccordionContent>
                                <ul className="flex flex-col gap-1.5">
                                    {segment.messages.map((message) => (
                                        <MessageItem key={message.id} message={message} />
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
function EndItem({ endedAt }: { endedAt: string }) {
    return (
        <li className="flex gap-3">
            <span className="flex flex-col items-center pt-1" aria-hidden>
                <span className="size-3 shrink-0 rounded-full bg-alerta-verde ring-4 ring-white" />
            </span>
            <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-center gap-2">
                    <Badge className="gap-1.5 bg-emerald-50 font-semibold text-emerald-700 ring-1 ring-emerald-200">
                        <span className="size-1.5 rounded-full bg-alerta-verde" aria-hidden />
                        Normal
                    </Badge>
                    <span className="text-xs text-gray-500">La alerta terminó</span>
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
}

/** Colores por los que pasó la alerta hasta volver a lo normal, con fechas, duración y mensajes (RN-20). */
export function EventTimeline({ levels, endedAt }: EventTimelineProps) {
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
                            Duración acumulada: <strong className="font-data-mono text-gray-900">{formatDuration(totalDurationMs(levels, now))}</strong>
                        </>
                    )
                }
            />
            {levels.length === 0 ? (
                <p className="text-sm text-gray-500">Todavía no hay cambios de nivel registrados.</p>
            ) : (
                <ol className="flex flex-col rounded-lg border border-border p-3">
                    {levels.map((segment, index) => (
                        <SegmentItem
                            key={segment.seq}
                            segment={segment}
                            isLast={index === levels.length - 1 && endedAt === null}
                            now={now}
                        />
                    ))}
                    {endedAt && <EndItem endedAt={endedAt} />}
                </ol>
            )}
        </section>
    );
}
