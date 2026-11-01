import type { AlertNotificationEvent } from "@/services/realtime";
import { LATE_NOTICE_LABEL } from "../config/alertManagementDisplay";

const NOTIFICATION_ICON = "/favicon.svg";

let titlePrefix = "";

/**
 * Prefijo del título de toda notificación (la demo usa "[SIMULACIÓN] "): la notificación sale de la pestaña,
 * así que debe distinguirse sola de un aviso real.
 */
export function setBrowserNotificationTitlePrefix(prefix: string) {
    titlePrefix = prefix;
}

/** Permiso de notificaciones del navegador; `unsupported` si el navegador no tiene la Notification API. */
export type BrowserNotificationPermission = NotificationPermission | "unsupported";

export function browserNotificationPermission(): BrowserNotificationPermission {
    return typeof Notification === "undefined" ? "unsupported" : Notification.permission;
}

/**
 * Pide el permiso. Solo se llama desde un clic del usuario (botón del indicador de conexión), nunca al cargar
 * la app: los navegadores bloquean o penalizan los pedidos que no vienen de una interacción.
 */
export async function requestBrowserNotificationPermission(): Promise<BrowserNotificationPermission> {
    if (typeof Notification === "undefined") return "unsupported";
    return Notification.requestPermission();
}

/** Solo con permiso y con la pestaña oculta: con la pestaña a la vista ya se ve el banner. */
export function shouldShowBrowserNotification(permission: BrowserNotificationPermission, tabHidden: boolean): boolean {
    return permission === "granted" && tabHidden;
}

/**
 * Título y opciones de la notificación. `tag = alert_code`: un aviso nuevo de la misma alerta (p. ej. una
 * subida de nivel) reemplaza al anterior en el sistema operativo en vez de apilarse, y si hay varias pestañas
 * ocultas, la notificación de cada una reemplaza a la de las otras.
 */
export function buildBrowserNotification(notice: AlertNotificationEvent): { title: string; options: NotificationOptions } {
    return {
        title: titlePrefix + (notice.late ? `${notice.title} (${LATE_NOTICE_LABEL})` : notice.title),
        options: {
            body: notice.body,
            tag: notice.alert_code,
            icon: NOTIFICATION_ICON,
            // Rojo se queda en pantalla hasta que alguien lo atienda; los demás siguen la duración del sistema.
            requireInteraction: notice.level === "ROJO",
        },
    };
}

/**
 * Muestra la notificación si corresponde. Al hacer clic, trae la pestaña al frente y ejecuta `onClick`
 * (abrir el detalle de la alerta).
 */
export function showBrowserNotification(notice: AlertNotificationEvent, onClick: () => void) {
    if (!shouldShowBrowserNotification(browserNotificationPermission(), document.hidden)) return;

    const { title, options } = buildBrowserNotification(notice);
    try {
        const notification = new Notification(title, options);
        notification.onclick = () => {
            window.focus();
            onClick();
            notification.close();
        };
    } catch {
        // Algunos navegadores móviles solo permiten notificaciones desde un service worker: queda el banner.
    }
}
