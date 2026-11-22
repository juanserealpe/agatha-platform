import { useEffect, useRef, useState } from "react";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { cn } from "@/lib/utils";
import { EVENT_ICON, EVENT_LABEL, levelStyle } from "../../config/alertManagementDisplay";
import { memberCurrentLevel, type GroupCandidate } from "../../lib/alertGroup";
import { splitTabs } from "../../lib/alertTabs";
import { END_LABEL } from "../../lib/timeStrip";
import { FOCUS_CLASS } from "./timeStripStyles";

/** Alto fijo de la fila de pestañas: la franja fija lo suma a su alto y a la compensación del desplazamiento. */
export const TAB_LIST_HEIGHT_PX = 52;

const NOVELTY_LABEL = "Cambió desde que la vio";

interface TabIds {
    tabId: (alertId: number) => string;
    panelId: (alertId: number) => string;
}

interface AlertTabsProps extends TabIds {
    /** Alertas del grupo en orden cronológico (más de una). */
    members: readonly GroupCandidate[];
    activeId: number;
    /** Pestañas no abiertas cuyo estado o nivel cambió mientras el modal está abierto (spec §6). */
    novelty: ReadonlySet<number>;
    onSelect: (alertId: number) => void;
    onPrefetch: (alertId: number) => void;
}

/** Nivel actual de la alerta (activa) o "Finalizada". El texto acompaña al color (no depende solo del color). */
function TabState({ member }: { member: GroupCandidate }) {
    const level = memberCurrentLevel(member);
    if (!level) return <span>{END_LABEL}</span>;
    const style = levelStyle(level);
    return (
        <span className="flex items-center gap-1">
            <span className={cn("size-1.5 rounded-full", style.dotClass)} aria-hidden />
            {style.label}
        </span>
    );
}

/**
 * Tipo de alerta arriba; código y nivel (o "Finalizada") abajo. En celular, para que quepan las 3 pestañas: ícono y
 * código arriba, nivel abajo (el tipo queda para el lector de pantalla). Igual en la pestaña y en el menú "+N".
 */
function TabLabel({ member, novelty }: { member: GroupCandidate; novelty: boolean }) {
    const Icon = EVENT_ICON[member.event];
    return (
        <span className="flex min-w-0 flex-col gap-0.5 text-left">
            <span className="flex min-w-0 items-center gap-1.5 text-sm">
                <Icon className="size-3.5 shrink-0" aria-hidden />
                <span className="truncate max-sm:sr-only">{EVENT_LABEL[member.event]}</span>
                <span className="font-data-mono text-xs sm:hidden">{member.code}</span>
                {novelty && (
                    <span className="size-2 shrink-0 rounded-full bg-gray-700" title={NOVELTY_LABEL}>
                        <span className="sr-only">{NOVELTY_LABEL}</span>
                    </span>
                )}
            </span>
            <span className="flex items-center gap-1 font-data-mono text-xs whitespace-nowrap text-gray-600">
                <span className="max-sm:hidden">{member.code} ·</span>
                <TabState member={member} />
            </span>
        </span>
    );
}

interface OverflowMenuProps {
    members: readonly GroupCandidate[];
    novelty: ReadonlySet<number>;
    onSelect: (alertId: number) => void;
    onPrefetch: (alertId: number) => void;
}

/** "+N": las alertas del grupo que no caben como pestaña (hasta 3 visibles). La franja sí las muestra todas. */
function OverflowMenu({ members, novelty, onSelect, onPrefetch }: OverflowMenuProps) {
    const [open, setOpen] = useState(false);
    const anyNovelty = members.some((member) => novelty.has(member.id));
    return (
        <Popover open={open} onOpenChange={setOpen}>
            <PopoverTrigger
                aria-label={`${members.length} alertas más del punto`}
                className={cn(
                    "relative mb-1.5 flex h-9 shrink-0 cursor-pointer items-center rounded-md px-2.5 font-data-mono text-sm font-semibold text-gray-700 hover:bg-background",
                    FOCUS_CLASS
                )}
            >
                +{members.length}
                {anyNovelty && <span className="absolute top-1 right-1 size-2 rounded-full bg-gray-700" aria-hidden />}
            </PopoverTrigger>
            <PopoverContent align="end" className="w-64 gap-1 p-1">
                {members.map((member) => (
                    <button
                        key={member.id}
                        type="button"
                        onClick={() => {
                            setOpen(false);
                            onSelect(member.id);
                        }}
                        onPointerEnter={() => onPrefetch(member.id)}
                        className={cn("cursor-pointer rounded-md px-2.5 py-1.5 text-gray-700 hover:bg-muted", FOCUS_CLASS)}
                    >
                        <TabLabel member={member} novelty={novelty.has(member.id)} />
                    </button>
                ))}
            </PopoverContent>
        </Popover>
    );
}

/**
 * Pestañas de la vista combinada: una por alerta del grupo (spec §6, A2). Patrón de pestañas de WAI-ARIA con
 * activación automática: las flechas, Inicio y Fin cambian de pestaña.
 */
export function AlertTabs({ members, activeId, novelty, onSelect, onPrefetch, tabId, panelId }: AlertTabsProps) {
    const { visible, overflow } = splitTabs(members, activeId);
    const listRef = useRef<HTMLDivElement>(null);
    const activeTabId = tabId(activeId);

    // En pantallas angostas las pestañas se desplazan de lado: la abierta siempre queda a la vista (sin mover el modal).
    useEffect(() => {
        const list = listRef.current;
        const tab = list?.querySelector<HTMLElement>(`#${CSS.escape(activeTabId)}`);
        if (!list || !tab) return;
        const left = tab.offsetLeft;
        const right = left + tab.offsetWidth;
        if (left < list.scrollLeft) list.scrollLeft = left;
        else if (right > list.scrollLeft + list.clientWidth) list.scrollLeft = right - list.clientWidth;
    }, [activeTabId]);

    function handleKeyDown(event: React.KeyboardEvent, index: number) {
        const last = visible.length - 1;
        const next =
            event.key === "ArrowRight" ? (index === last ? 0 : index + 1)
            : event.key === "ArrowLeft" ? (index === 0 ? last : index - 1)
            : event.key === "Home" ? 0
            : event.key === "End" ? last
            : null;
        if (next === null) return;
        event.preventDefault();
        const target = visible[next];
        onSelect(target.id);
        listRef.current?.querySelector<HTMLElement>(`#${CSS.escape(tabId(target.id))}`)?.focus();
    }

    return (
        // Línea inferior pintada como sombra interior: la pestaña abierta (blanca) la tapa y se une al contenido.
        <div className="flex items-end gap-1 shadow-[inset_0_-1px_0_var(--color-border)]" style={{ height: TAB_LIST_HEIGHT_PX }}>
            <div ref={listRef} role="tablist" aria-label="Alertas del punto" className="relative flex min-w-0 flex-1 items-end gap-1 overflow-x-auto overflow-y-hidden [scrollbar-width:thin] max-sm:[scrollbar-width:none]">
                {visible.map((member, index) => {
                    const active = member.id === activeId;
                    return (
                        <button
                            key={member.id}
                            id={tabId(member.id)}
                            type="button"
                            role="tab"
                            aria-selected={active}
                            aria-controls={panelId(member.id)}
                            tabIndex={active ? 0 : -1}
                            onClick={() => onSelect(member.id)}
                            onKeyDown={(event) => handleKeyDown(event, index)}
                            onPointerEnter={() => onPrefetch(member.id)}
                            // Pestaña de carpeta: la abierta es blanca como el contenido de su alerta y se une a él;
                            // las demás quedan sobre el gris de la zona del punto.
                            className={cn(
                                "flex h-12 max-w-56 min-w-0 shrink-0 cursor-pointer items-center rounded-t-lg border border-b-0 px-3 max-sm:px-2",
                                FOCUS_CLASS,
                                "focus-visible:ring-inset focus-visible:ring-offset-0",
                                active
                                    ? "border-border bg-background font-semibold text-gray-900"
                                    : "border-transparent text-gray-700 hover:bg-background/60"
                            )}
                        >
                            <TabLabel member={member} novelty={novelty.has(member.id)} />
                        </button>
                    );
                })}
            </div>
            {overflow.length > 0 && <OverflowMenu members={overflow} novelty={novelty} onSelect={onSelect} onPrefetch={onPrefetch} />}
        </div>
    );
}
