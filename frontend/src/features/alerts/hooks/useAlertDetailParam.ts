import { useCallback } from "react";
import { useLocation, useNavigate, useSearchParams } from "react-router";
import { DEFAULT_MANAGEMENT_MODE, parseManagementMode, type ManagementMode } from "../lib/managementMode";

export const ALERT_DETAIL_PARAM = "alerta";
export const MANAGEMENT_MODE_PARAM = "gestion";

// Marca en el history state: el modal se abrió desde la app (no por un enlace directo).
const OPENED_IN_APP_STATE = { alertDetailOpenedInApp: true };

function wasOpenedInApp(state: unknown): boolean {
    return typeof state === "object" && state !== null && "alertDetailOpenedInApp" in state;
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
                // Si ya hay un detalle abierto se reemplaza, para que "Atrás" no recorra cada alerta vista.
                { replace: alertId !== null, state: OPENED_IN_APP_STATE }
            );
        },
        [alertId, setSearchParams]
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

    return { alertId, mode, open, close };
}
