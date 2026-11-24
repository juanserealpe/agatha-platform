import type { AlertLevel, Classification } from "@/services/alertManagement";
import type { AlertNotificationEvent } from "@/services/realtime";
import { LEVEL_ORDER } from "../config/alertManagementDisplay";
import type { RowAction } from "./rowActions";

/** Banners de alerta nueva que se muestran a la vez; el resto se resume como "+N". */
export const MAX_VISIBLE_NOTICES = 3;

/** Tras volver a la normalidad, el banner queda este tiempo (para que se vea que terminó) y se cierra solo. */
export const ENDED_NOTICE_TTL_MS = 60_000;

/**
 * Aviso en pantalla. `ended`: la alerta ya finalizó (volvió a la normalidad). `previous_level`: nivel del banner
 * que este aviso reemplazó, para mostrar el cambio (el evento del stream no lo trae).
 */
export type Notice = AlertNotificationEvent & { ended?: boolean; previous_level?: AlertLevel };

export interface NewAlertsState {
    /** Avisos ya recibidos: tras una reconexión el servidor puede reenviarlos y no deben repetirse. */
    seenMessageIds: ReadonlySet<number>;
    /** Un aviso por alerta (el más reciente), pendiente de que el usuario lo vea o descarte. */
    notices: Notice[];
}

export const INITIAL_NEW_ALERTS_STATE: NewAlertsState = { seenMessageIds: new Set(), notices: [] };

/**
 * Incorpora un aviso del stream. Se identifica por `message_id`, no por alerta: una subida de nivel conserva
 * el id de la alerta y también se avisa. Un aviso nuevo de la misma alerta reemplaza al anterior en vez de
 * apilarse (una reactivación reemplaza también al banner de "Volvió a la normalidad").
 */
export function ingestNotification(state: NewAlertsState, notice: AlertNotificationEvent): NewAlertsState {
    if (state.seenMessageIds.has(notice.message_id)) return state;

    const replaced = state.notices.find((current) => current.alert_code === notice.alert_code);
    const next: Notice =
        replaced && replaced.level !== notice.level ? { ...notice, previous_level: replaced.level } : notice;
    return {
        seenMessageIds: new Set([...state.seenMessageIds, notice.message_id]),
        notices: [...state.notices.filter((current) => current.alert_code !== notice.alert_code), next],
    };
}

export interface LevelChange {
    /** null: no hubo cambio de nivel o no se sabe desde cuál (p. ej. un Rojo sin banner previo). */
    from: AlertLevel | null;
    to: AlertLevel;
}

/**
 * Cambio de nivel que anuncia el aviso. Solo las subidas tienen origen: el del banner reemplazado o, en
 * `UP_NARANJA_*`, Amarillo (es el único nivel por debajo). Finalizada, se muestra el máximo que alcanzó.
 */
export function noticeLevelChange(notice: Notice): LevelChange {
    if (notice.ended) return { from: null, to: notice.max_level };
    if (!notice.code.startsWith("UP_")) return { from: null, to: notice.level };
    const fallback: AlertLevel | null = notice.code.startsWith("UP_NARANJA") ? "AMARILLO" : null;
    return { from: notice.previous_level ?? fallback, to: notice.level };
}

/** Reemplaza el banner de esa alerta con `update`. Misma referencia si no hay banner o `update` no cambia nada. */
function updateNotice(state: NewAlertsState, alertCode: string, update: (notice: Notice) => Notice): NewAlertsState {
    const index = state.notices.findIndex((notice) => notice.alert_code === alertCode);
    if (index === -1) return state;

    const next = update(state.notices[index]);
    if (next === state.notices[index]) return state;
    const notices = [...state.notices];
    notices[index] = next;
    return { ...state, notices };
}

/**
 * La alerta se clasificó (aquí, en otra pestaña o desde la app): su banner guarda la clasificación y deja de
 * titilar. Misma referencia si no hay banner de esa alerta o ya la tenía.
 */
export function classifyNotice(state: NewAlertsState, alertCode: string, classification: Classification): NewAlertsState {
    return updateNotice(state, alertCode, (notice) =>
        notice.classification === classification ? notice : { ...notice, classification }
    );
}

/** La alerta finalizó: su banner pasa a "Volvió a la normalidad". Misma referencia si no hay banner o ya lo estaba. */
export function endNotice(state: NewAlertsState, alertCode: string): NewAlertsState {
    return updateNotice(state, alertCode, (notice) => (notice.ended ? notice : { ...notice, ended: true }));
}

/** Cierra el banner solo si sigue finalizado con ese aviso: si la alerta se reactivó, el banner nuevo se queda. */
export function expireEndedNotice(state: NewAlertsState, alertCode: string, messageId: number): NewAlertsState {
    const notice = state.notices.find((current) => current.alert_code === alertCode);
    if (!notice?.ended || notice.message_id !== messageId) return state;
    return dismissNotice(state, alertCode);
}

export function dismissNotice(state: NewAlertsState, alertCode: string): NewAlertsState {
    return { ...state, notices: state.notices.filter((notice) => notice.alert_code !== alertCode) };
}

/**
 * Qué hace el clic en el banner: sin clasificar, abre directo en la clasificación (es lo que se espera del
 * operador); ya clasificada, en lectura. El aviso no trae `available_actions`: si el backend ya no permite
 * clasificar (p. ej. se gestionó en otra pestaña), el modal cae solo al resumen de lectura.
 */
export function noticeAction(notice: Notice): Extract<RowAction, "classify" | "view"> {
    return notice.classification === null ? "classify" : "view";
}

/** Orden de los banners: los de alertas en curso primero; luego mayor nivel y, entre iguales, el más reciente. */
export function sortNotices(notices: readonly Notice[]): Notice[] {
    return [...notices].sort(
        (a, b) =>
            Number(a.ended ?? false) - Number(b.ended ?? false) ||
            LEVEL_ORDER.indexOf(b.level) - LEVEL_ORDER.indexOf(a.level) ||
            Date.parse(b.occurred_at) - Date.parse(a.occurred_at)
    );
}
