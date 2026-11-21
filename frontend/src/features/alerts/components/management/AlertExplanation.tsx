import { useId } from "react";
import { Info } from "lucide-react";
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";
import type { AlertDetail, AlertLevel, TriggerVariable } from "@/services/alertManagement";
import {
    ANOMALY_SCORE_NOTE,
    EVENT_EXPLANATION,
    formatAnomalyScore,
    leakSignatureText,
    SCORE_LEVEL_CAPTION,
} from "../../config/alertExplanations";
import { LevelBadge } from "../badges/LevelBadge";
import { StepHeading } from "./StepHeading";

type ExplanationDetail = Pick<
    AlertDetail,
    | "event"
    | "max_level"
    | "current_level"
    | "in_progress"
    | "opening_indicator_value"
    | "last_indicator_value"
    | "levels"
    | "trigger_variables"
    | "leak_signature"
>;

interface ScoreProps {
    label: string;
    value: number;
    level: AlertLevel;
    /** Aclara que el chip es el nivel de la alerta, no el del valor (§9.2). */
    levelCaption: string;
}

function Score({ label, value, level, levelCaption }: ScoreProps) {
    return (
        <div className="flex flex-col gap-1 rounded-lg bg-gray-50 px-3 py-2">
            <dt className="text-xs text-gray-500">{label}</dt>
            <dd className="flex flex-col gap-1">
                <span className="font-data-mono text-lg font-bold text-gray-900">{formatAnomalyScore(value)}</span>
                <span className="flex flex-wrap items-center gap-1.5 text-xs text-gray-600">
                    {levelCaption} <LevelBadge level={level} />
                </span>
            </dd>
        </div>
    );
}

function AnomalyScores({ detail }: { detail: ExplanationDetail }) {
    const openingLevel = detail.levels[0]?.level ?? detail.max_level;
    const lastLevel = detail.current_level ?? detail.levels.at(-1)?.level ?? detail.max_level;
    return (
        <>
            <dl className="grid grid-cols-1 gap-2 sm:grid-cols-2">
                <Score
                    label="Nivel de anomalía al abrir"
                    value={detail.opening_indicator_value}
                    level={openingLevel}
                    levelCaption={SCORE_LEVEL_CAPTION.opening}
                />
                <Score
                    label="Nivel de anomalía en la última hora"
                    value={detail.last_indicator_value}
                    level={lastLevel}
                    levelCaption={detail.in_progress ? SCORE_LEVEL_CAPTION.lastInProgress : SCORE_LEVEL_CAPTION.lastFinished}
                />
            </dl>
            <p className="flex items-start gap-1.5 text-xs text-gray-500">
                <Info className="mt-0.5 size-3.5 shrink-0" aria-hidden />
                {ANOMALY_SCORE_NOTE}
            </p>
        </>
    );
}

function isOutOfRange(variable: TriggerVariable): boolean {
    return variable.value < variable.normal_min || variable.value > variable.normal_max;
}

// Propuesta del frontend: el backend AÚN NO envía trigger_variables. Sin ellas la sección no muestra nada aquí.
function TriggerVariables({ variables }: { variables: TriggerVariable[] }) {
    const outOfRange = variables.filter(isOutOfRange);
    if (outOfRange.length === 0) return null;

    return (
        <div className="flex flex-col gap-1.5">
            <p className="text-xs font-medium text-gray-600">Mediciones fuera de su rango normal</p>
            <ul className="flex flex-col gap-1">
                {outOfRange.map((variable) => (
                    <li key={variable.key} className="flex flex-wrap items-baseline justify-between gap-2 text-sm">
                        <span className="text-gray-800">{variable.name}</span>
                        <span className="font-data-mono text-xs text-gray-600">
                            <strong className="text-gray-900">
                                {formatAnomalyScore(variable.value)} {variable.unit}
                            </strong>{" "}
                            (normal: {formatAnomalyScore(variable.normal_min)}–{formatAnomalyScore(variable.normal_max)} {variable.unit})
                        </span>
                    </li>
                ))}
            </ul>
        </div>
    );
}

interface AlertExplanationProps {
    detail: ExplanationDetail;
    /** Plegada: el título abre y cierra la explicación (pasos de clasificar y etiquetar). */
    collapsed?: boolean;
}

const SECTION_TITLE = "¿Por qué se generó esta alerta?";

/** Explicación en lenguaje claro: qué evento es y cuánto se aleja la medición de lo normal. */
export function AlertExplanation({ detail, collapsed = false }: AlertExplanationProps) {
    if (!collapsed) {
        return <ExplanationSection detail={detail} />;
    }
    return (
        <Accordion>
            <AccordionItem value="explanation" className="rounded-lg border border-border px-3">
                <AccordionTrigger className="py-2.5 font-data-mono text-xs font-bold tracking-wider text-gray-900 uppercase hover:no-underline">
                    {SECTION_TITLE}
                </AccordionTrigger>
                <AccordionContent>
                    <ExplanationBody detail={detail} />
                </AccordionContent>
            </AccordionItem>
        </Accordion>
    );
}

function ExplanationSection({ detail }: { detail: ExplanationDetail }) {
    const titleId = useId();
    return (
        <section aria-labelledby={titleId} className="flex flex-col gap-3">
            <StepHeading id={titleId} title={SECTION_TITLE} />
            <div className="rounded-lg border border-border p-3">
                <ExplanationBody detail={detail} />
            </div>
        </section>
    );
}

function ExplanationBody({ detail }: { detail: ExplanationDetail }) {
    const event = EVENT_EXPLANATION[detail.event];
    const levelPhrase = event.levelPhrase[detail.max_level];
    const leak = detail.event === "FUGA" ? leakSignatureText(detail.leak_signature) : null;

    return (
        <div className="flex flex-col gap-3">
            <p className="text-sm text-gray-800">
                <strong className="text-gray-900">{event.title}.</strong> {event.description}
            </p>
            {levelPhrase && <p className="text-sm text-gray-800">{levelPhrase}</p>}
            {leak && (
                <p className="text-sm text-gray-800">
                    <strong className="text-gray-900">{leak.title}.</strong> {leak.text}
                </p>
            )}

            {/* Sin puntaje propio (movimiento) se oculta el bloque: nunca se muestra un 0 que no es una medición. */}
            {event.hasAnomalyScore && <AnomalyScores detail={detail} />}

            {detail.trigger_variables && <TriggerVariables variables={detail.trigger_variables} />}
        </div>
    );
}
