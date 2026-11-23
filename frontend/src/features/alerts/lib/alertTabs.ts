import { memberCurrentLevel, type GroupCandidate } from "./alertGroup";

/**
 * Pestañas de la vista combinada (etapa 5, "Diseño del adaptador" en progreso_multiindicador.md): una pestaña por
 * alerta del grupo. Se ven hasta `MAX_VISIBLE_TABS`; las demás van en el menú "+N". La franja siempre muestra todas.
 */
export const MAX_VISIBLE_TABS = 3;

type TabMember = Pick<GroupCandidate, "id" | "state" | "started_at" | "ended_at" | "levels" | "max_level">;

export interface TabSplit<T> {
    /** En orden cronológico (el del grupo). */
    visible: T[];
    /** En orden cronológico. Vacío si caben todas. */
    overflow: T[];
}

/**
 * Qué pestañas se ven: siempre la abierta; luego las activas, de la más reciente a la más antigua; si sobra lugar,
 * las finalizadas más recientes. `members` llega en orden cronológico (como lo arma el grupo) y así se devuelve.
 */
export function splitTabs<T extends TabMember>(members: readonly T[], activeId: number, max: number = MAX_VISIBLE_TABS): TabSplit<T> {
    if (members.length <= max) return { visible: [...members], overflow: [] };
    const byRecency = [...members].sort((a, b) => new Date(b.started_at).getTime() - new Date(a.started_at).getTime());
    const priority = [
        ...members.filter((member) => member.id === activeId),
        ...byRecency.filter((member) => member.id !== activeId && member.state === "ACTIVA"),
        ...byRecency.filter((member) => member.id !== activeId && member.state !== "ACTIVA"),
    ];
    const shown = new Set(priority.slice(0, max).map((member) => member.id));
    return {
        visible: members.filter((member) => shown.has(member.id)),
        overflow: members.filter((member) => !shown.has(member.id)),
    };
}

/** Lo que, si cambia mientras el modal está abierto, enciende el punto de novedad de la pestaña (spec §6). */
export function tabSignature(member: TabMember): string {
    return `${member.state}|${memberCurrentLevel(member) ?? member.max_level}`;
}

/**
 * Pestañas con novedad: las que no están abiertas y cambiaron de estado o de nivel desde que se vieron, o que
 * aparecieron después de abrir el modal (`seen` no las tiene).
 */
export function tabsWithNovelty(members: readonly TabMember[], seen: ReadonlyMap<number, string>, activeId: number): Set<number> {
    return new Set(
        members
            .filter((member) => member.id !== activeId && seen.get(member.id) !== tabSignature(member))
            .map((member) => member.id)
    );
}
