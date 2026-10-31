import type { AlertNotificationEvent } from "@/services/realtime";
import { LEVEL_ORDER } from "../config/alertManagementDisplay";

/** Banners de alerta nueva que se muestran a la vez; el resto se resume como "+N". */
export const MAX_VISIBLE_NOTICES = 3;

export interface NewAlertsState {
    /** Avisos ya recibidos: tras una reconexión el servidor puede reenviarlos y no deben repetirse. */
    seenMessageIds: ReadonlySet<number>;
    /** Un aviso por alerta (el más reciente), pendiente de que el usuario lo vea o descarte. */
    notices: AlertNotificationEvent[];
}

export const INITIAL_NEW_ALERTS_STATE: NewAlertsState = { seenMessageIds: new Set(), notices: [] };

/**
 * Incorpora un aviso del stream. Se identifica por `message_id`, no por alerta: una subida de nivel conserva
 * el id de la alerta y también se avisa. Un aviso nuevo de la misma alerta reemplaza al anterior en vez de
 * apilarse.
 */
export function ingestNotification(state: NewAlertsState, notice: AlertNotificationEvent): NewAlertsState {
    if (state.seenMessageIds.has(notice.message_id)) return state;

    return {
        seenMessageIds: new Set([...state.seenMessageIds, notice.message_id]),
        notices: [...state.notices.filter((current) => current.alert_code !== notice.alert_code), notice],
    };
}

export function dismissNotice(state: NewAlertsState, alertCode: string): NewAlertsState {
    return { ...state, notices: state.notices.filter((notice) => notice.alert_code !== alertCode) };
}

/** Orden de los banners: mayor nivel primero y, entre iguales, el aviso más reciente. */
export function sortNotices(notices: readonly AlertNotificationEvent[]): AlertNotificationEvent[] {
    return [...notices].sort(
        (a, b) =>
            LEVEL_ORDER.indexOf(b.level) - LEVEL_ORDER.indexOf(a.level) ||
            Date.parse(b.occurred_at) - Date.parse(a.occurred_at)
    );
}
