import { useCallback } from "react";
import { useLocation, useNavigate, useSearchParams } from "react-router";
import { DEFAULT_MANAGEMENT_MODE, parseManagementMode, type ManagementMode } from "../lib/managementMode";

export const ALERT_DETAIL_PARAM = "alerta";
export const MANAGEMENT_MODE_PARAM = "gestion";

// Marca en el history state: el modal se abrió desde la app (no por un enlace directo).
const OPENED_IN_APP_STATE = { alertDetailOpenedInApp: true };

// Marca en el history state: el último cambio de ?alerta fue un cambio de pestaña de la vista combinada (el modal
// sigue abierto con su estado; ver AlertDetailHost).
const TAB_SWITCH_KEY = "alertDetailTabSwitch";

function wasOpenedInApp(state: unknown): boolean {
    return typeof state === "object" && state !== null && "alertDetailOpenedInApp" in state;
}

function isTabSwitch(state: unknown): boolean {
    return typeof state === "object" && state !== null && TAB_SWITCH_KEY in state;
}

/**
 * Única forma de abrir y cerrar el modal de una alerta. El estado vive en la URL (?alerta=<id>&gestion=<modo>),
 * así el modal se puede compartir por enlace y funciona igual desde cualquier página.
 */
export function useAlertDetailParam() {
    const [searchParams, setSearchParams] = useSearchParams();
    const location = useLocation();
    const navigate = useNavigate();
    const alertId = searchParams.get(ALERT_DETAIL_PARAM);
    const mode = parseManagementMode(searchParams.get(MANAGEMENT_MODE_PARAM));

    const open = useCallback(
        (id: string, nextMode: ManagementMode = DEFAULT_MANAGEMENT_MODE) => {
            setSearchParams(
                (previous) => {
                    const next = new URLSearchParams(previous);
                    next.set(ALERT_DETAIL_PARAM, id);
                    // Lectura es el modo por defecto: no se escribe para que el enlace quede limpio.
                    if (nextMode === DEFAULT_MANAGEMENT_MODE) next.delete(MANAGEMENT_MODE_PARAM);
                    else next.set(MANAGEMENT_MODE_PARAM, nextMode);
                    return next;
                },
                // Si ya hay un detalle abierto se reemplaza, para que "Atrás" no recorra cada alerta vista. Se lee
                // de la URL actual (no del render) para que `open` sea estable y las filas memoizadas no se repinten.
                { replace: new URLSearchParams(window.location.search).has(ALERT_DETAIL_PARAM), state: OPENED_IN_APP_STATE }
            );
        },
        [setSearchParams]
    );

    // Pestaña de la vista combinada: la URL lleva el id de la pestaña abierta (un enlace abre el grupo en esa
    // pestaña). Siempre reemplaza, para que "Atrás" no recorra las pestañas, y vuelve a lectura (el modo de gestión
    // era de la alerta con que se abrió el modal). Conserva la marca de "abierto desde la app" para cerrar bien.
    const selectTab = useCallback(
        (id: string) => {
            setSearchParams(
                (previous) => {
                    const next = new URLSearchParams(previous);
                    next.set(ALERT_DETAIL_PARAM, id);
                    next.delete(MANAGEMENT_MODE_PARAM);
                    return next;
                },
                {
                    replace: true,
                    state: { ...(wasOpenedInApp(location.state) ? OPENED_IN_APP_STATE : {}), [TAB_SWITCH_KEY]: true },
                }
            );
        },
        [location.state, setSearchParams]
    );

    const close = useCallback(() => {
        // Abierto desde la app: "Atrás" deja el historial como estaba antes de abrirlo.
        // Abierto por enlace directo: se quitan los parámetros sin salir de la app.
        if (wasOpenedInApp(location.state)) {
            navigate(-1);
            return;
        }
        setSearchParams(
            (previous) => {
                const next = new URLSearchParams(previous);
                next.delete(ALERT_DETAIL_PARAM);
                next.delete(MANAGEMENT_MODE_PARAM);
                return next;
            },
            { replace: true }
        );
    }, [location.state, navigate, setSearchParams]);

    return { alertId, mode, open, close, selectTab, tabSwitch: isTabSwitch(location.state) };
}
