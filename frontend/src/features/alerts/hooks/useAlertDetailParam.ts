import { useCallback } from "react";
import { useLocation, useNavigate, useSearchParams } from "react-router";

export const ALERT_DETAIL_PARAM = "alerta";

// Marca en el history state: el modal se abrió desde la app (no por un enlace directo).
const OPENED_IN_APP_STATE = { alertDetailOpenedInApp: true };

function wasOpenedInApp(state: unknown): boolean {
    return typeof state === "object" && state !== null && "alertDetailOpenedInApp" in state;
}

/**
 * Única forma de abrir y cerrar el detalle de una alerta. El estado vive en la URL (?alerta=<id>),
 * así el detalle se puede compartir por enlace y funciona igual desde cualquier página.
 */
export function useAlertDetailParam() {
    const [searchParams, setSearchParams] = useSearchParams();
    const location = useLocation();
    const navigate = useNavigate();
    const alertId = searchParams.get(ALERT_DETAIL_PARAM);

    const open = useCallback(
        (id: string) => {
            setSearchParams(
                (previous) => {
                    const next = new URLSearchParams(previous);
                    next.set(ALERT_DETAIL_PARAM, id);
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
        // Abierto por enlace directo: se quita el parámetro sin salir de la app.
        if (wasOpenedInApp(location.state)) {
            navigate(-1);
            return;
        }
        setSearchParams(
            (previous) => {
                const next = new URLSearchParams(previous);
                next.delete(ALERT_DETAIL_PARAM);
                return next;
            },
            { replace: true }
        );
    }, [location.state, navigate, setSearchParams]);

    return { alertId, open, close };
}
