import { useCallback, useId, useLayoutEffect, useRef, useState } from "react";
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

const NO_PHASES: ReadonlySet<number> = new Set();

/** Lo que usa la línea de tiempo de UNA pestaña (alerta). */
export interface TimelineNavigation {
    /** Fases con los mensajes desplegados. Solo cambian por clic del usuario o al navegar desde la franja. */
    expandedSeqs: ReadonlySet<number>;
    togglePhase: (seq: number, open: boolean) => void;
    /** Fase recién alcanzada desde la franja (resaltado breve). */
    arrivedSeq: number | null;
    /** Navegando desde la franja: las fases se pliegan sin animación. */
    navigating: boolean;
    /** Rango bajo el cursor en la línea de tiempo: la franja lo dibuja como banda (cursor de tiempo). */
    setHoverRange: (range: HoverRange | null) => void;
    phaseElementId: (seq: number) => string;
}

/** Estado compartido entre la franja y las pestañas. */
export interface GroupNavigation {
    /** Tramo marcado como seleccionado en la franja. */
    selected: PhaseRef | null;
    hoverRange: HoverRange | null;
    /** Si la fase es de otra alerta, primero cambia a su pestaña (CA6). */
    navigateToPhase: (target: PhaseRef) => void;
    forAlert: (alertId: number) => TimelineNavigation;
}

/**
 * Estado compartido entre la franja y la línea de tiempo de cada pestaña (spec multiindicador §5.3, §6 y §10.2):
 * fases desplegadas POR PESTAÑA (cada una recuerda las suyas mientras el modal esté abierto), tramo seleccionado y
 * cursor de tiempo. La franja solo navega: nada de esto cambia datos.
 *
 * `activeId` es la pestaña abierta y `selectTab` la cambia. Al navegar a otra pestaña, el desplazamiento espera a
 * que su fase esté en pantalla (su detalle puede estar cargando): lo hace el efecto de abajo.
 */
export function useTimelineNavigation(activeId: number, selectTab: (alertId: number) => void): GroupNavigation {
    const idPrefix = useId();
    const [expanded, setExpanded] = useState<ReadonlyMap<number, ReadonlySet<number>>>(() => new Map());
    const [selected, setSelected] = useState<PhaseRef | null>(null);
    const [arrived, setArrived] = useState<PhaseRef | null>(null);
    const [navigating, setNavigating] = useState(false);
    const [hoverRange, setHoverRange] = useState<HoverRange | null>(null);
    const timers = useRef<number[]>([]);
    // Fase de otra pestaña a la que se navega: se desplaza cuando aparece en pantalla.
    const pending = useRef<PhaseRef | null>(null);

    const elementId = useCallback((alertId: number, seq: number) => `${idPrefix}-a${alertId}-phase-${seq}`, [idPrefix]);

    const togglePhase = useCallback((alertId: number, seq: number, open: boolean) => {
        setExpanded((current) => {
            const next = new Set(current.get(alertId));
            if (open) next.add(seq);
            else next.delete(seq);
            return new Map(current).set(alertId, next);
        });
    }, []);

    const finishNavigation = useCallback((phase: HTMLElement) => {
        // El foco pasa a la fase para que el teclado y el lector de pantalla sigan desde ahí.
        phase.focus({ preventScroll: true });
        phase.scrollIntoView({ behavior: "smooth", block: "start" });
        timers.current = [
            window.setTimeout(() => setNavigating(false), NAVIGATION_NO_ANIMATION_MS),
            window.setTimeout(() => setArrived(null), ARRIVAL_HIGHLIGHT_MS),
        ];
    }, []);

    const navigateToPhase = useCallback(
        (target: PhaseRef) => {
            timers.current.forEach((timer) => window.clearTimeout(timer));
            const apply = () => {
                setNavigating(true);
                setExpanded((current) => new Map(current).set(target.alertId, new Set([target.seq])));
                setSelected(target);
                setArrived(target);
            };
            if (target.alertId !== activeId) {
                pending.current = target;
                apply();
                selectTab(target.alertId);
                return;
            }
            pending.current = null;
            // Se despliega la fase y se pliegan las demás (sin animación) antes de medir: así se desplaza una sola
            // vez y directo a su posición final.
            flushSync(apply);
            const phase = document.getElementById(elementId(target.alertId, target.seq));
            if (phase) finishNavigation(phase);
        },
        [activeId, elementId, finishNavigation, selectTab]
    );

    // Sin dependencias a propósito: corre tras cada render hasta que la fase de la otra pestaña esté visible.
    useLayoutEffect(() => {
        const target = pending.current;
        if (!target || target.alertId !== activeId) return;
        const phase = document.getElementById(elementId(target.alertId, target.seq));
        // offsetParent null: el panel todavía está oculto.
        if (!phase || phase.offsetParent === null) return;
        pending.current = null;
        finishNavigation(phase);
    });

    const forAlert = useCallback(
        (alertId: number): TimelineNavigation => ({
            expandedSeqs: expanded.get(alertId) ?? NO_PHASES,
            togglePhase: (seq, open) => togglePhase(alertId, seq, open),
            arrivedSeq: arrived?.alertId === alertId ? arrived.seq : null,
            navigating,
            setHoverRange,
            phaseElementId: (seq) => elementId(alertId, seq),
        }),
        [expanded, arrived, navigating, togglePhase, elementId]
    );

    return { selected, hoverRange, navigateToPhase, forAlert };
}
