import { useCallback, useId, useRef, useState } from "react";
import { flushSync } from "react-dom";
import type { HoverRange } from "../lib/timeStrip";

/** Tramo de la franja elegido para navegar: una fase (`seq`) de una alerta (`alertId`). */
export interface PhaseRef {
    alertId: number;
    seq: number;
}

// Resaltado breve de la fase a la que se llegó desde la franja (§5.3, paso 3).
const ARRIVAL_HIGHLIGHT_MS = 1600;
// Mientras dura la navegación las fases que se pliegan no animan: si lo hicieran, la de destino
// se movería durante el desplazamiento (bajaba hasta el primer mensaje y luego subía al título).
const NAVIGATION_NO_ANIMATION_MS = 400;

export interface TimelineNavigation {
    /** Fases con los mensajes desplegados. Solo cambian por clic del usuario o al navegar desde la franja. */
    expandedSeqs: ReadonlySet<number>;
    togglePhase: (seq: number, open: boolean) => void;
    /** Tramo marcado como seleccionado en la franja. */
    selected: PhaseRef | null;
    /** Fase recién alcanzada desde la franja (resaltado breve). */
    arrivedSeq: number | null;
    /** Navegando desde la franja: las fases se pliegan sin animación. */
    navigating: boolean;
    navigateToPhase: (target: PhaseRef) => void;
    /** Rango bajo el cursor en la línea de tiempo: la franja lo dibuja como banda (cursor de tiempo). */
    hoverRange: HoverRange | null;
    setHoverRange: (range: HoverRange | null) => void;
    phaseElementId: (seq: number) => string;
}

/**
 * Estado compartido entre la franja y la línea de tiempo (spec multiindicador §5.3 y §10.2): fases
 * desplegadas, tramo seleccionado y cursor de tiempo. La franja solo navega: nada de esto cambia datos.
 * Las pestañas por alerta (etapa 5) agregarán aquí la pestaña activa.
 */
export function useTimelineNavigation(): TimelineNavigation {
    const idPrefix = useId();
    const [expandedSeqs, setExpandedSeqs] = useState<ReadonlySet<number>>(() => new Set());
    const [selected, setSelected] = useState<PhaseRef | null>(null);
    const [arrivedSeq, setArrivedSeq] = useState<number | null>(null);
    const [navigating, setNavigating] = useState(false);
    const [hoverRange, setHoverRange] = useState<HoverRange | null>(null);
    const timers = useRef<number[]>([]);

    const phaseElementId = useCallback((seq: number) => `${idPrefix}-phase-${seq}`, [idPrefix]);

    const togglePhase = useCallback((seq: number, open: boolean) => {
        setExpandedSeqs((current) => {
            const next = new Set(current);
            if (open) next.add(seq);
            else next.delete(seq);
            return next;
        });
    }, []);

    const navigateToPhase = useCallback(
        (target: PhaseRef) => {
            timers.current.forEach((timer) => window.clearTimeout(timer));
            // Se despliega la fase y se pliegan las demás (sin animación) antes de medir: así se desplaza una sola
            // vez y directo a su posición final.
            flushSync(() => {
                setNavigating(true);
                setExpandedSeqs(new Set([target.seq]));
                setSelected(target);
                setArrivedSeq(target.seq);
            });
            const phase = document.getElementById(phaseElementId(target.seq));
            // El foco pasa a la fase para que el teclado y el lector de pantalla sigan desde ahí.
            phase?.focus({ preventScroll: true });
            phase?.scrollIntoView({ behavior: "smooth", block: "start" });
            timers.current = [
                window.setTimeout(() => setNavigating(false), NAVIGATION_NO_ANIMATION_MS),
                window.setTimeout(() => setArrivedSeq(null), ARRIVAL_HIGHLIGHT_MS),
            ];
        },
        [phaseElementId]
    );

    return {
        expandedSeqs,
        togglePhase,
        selected,
        arrivedSeq,
        navigating,
        navigateToPhase,
        hoverRange,
        setHoverRange,
        phaseElementId,
    };
}
