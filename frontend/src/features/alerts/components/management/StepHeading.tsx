import type { ReactNode } from "react";

interface StepHeadingProps {
    id: string;
    /** Número del paso; sin él es un título de sección. */
    step?: number;
    title: string;
    /** Dato a la derecha, p. ej. "Confirmada por operador". */
    aside?: ReactNode;
}

export function StepHeading({ id, step, title, aside }: StepHeadingProps) {
    return (
        <div className="flex flex-wrap items-center justify-between gap-2">
            <h3 id={id} className="flex items-center gap-2 font-data-mono text-xs font-bold tracking-wider text-gray-900 uppercase">
                {step !== undefined && (
                    <span className="flex size-5 items-center justify-center rounded-full bg-gray-900 text-xs text-white" aria-hidden>
                        {step}
                    </span>
                )}
                {step !== undefined && <span className="sr-only">Paso {step}: </span>}
                {title}
            </h3>
            {aside && <div className="text-xs text-gray-500">{aside}</div>}
        </div>
    );
}
