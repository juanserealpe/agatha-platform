import { useEffect, useState, type ReactNode } from "react";
import { Navigate, useLocation, useNavigate } from "react-router";
import { useQueryClient } from "@tanstack/react-query";
import { tokenStore, verifySession } from "@/services/auth";
import { SESSION_EXPIRED_EVENT } from "@/services/http";
import { ROUTES } from "./routes";
import { loginRedirectState } from "./sessionRedirect";

/**
 * Guard de la app real (la demo no lo monta): sin sesión redirige a `/login` recordando la página pedida.
 * Sigue al `tokenStore`, así que cerrar sesión (en esta u otra pestaña) desmonta la app y con ella el stream.
 */
export function RequireSession({ children }: { children: ReactNode }) {
    const [session, setSession] = useState(() => tokenStore.get());
    const location = useLocation();
    const navigate = useNavigate();
    const queryClient = useQueryClient();

    useEffect(() => tokenStore.subscribe(setSession), []);

    // Al arrancar con un token guardado: detecta un secreto rotado en el servidor (el 401 limpia la sesión).
    useEffect(() => {
        void verifySession();
    }, []);

    // Un 401 en cualquier petición: fuera los datos del admin y al login.
    useEffect(() => {
        const onExpired = () => {
            queryClient.clear();
            navigate(ROUTES.login, { replace: true });
        };
        window.addEventListener(SESSION_EXPIRED_EVENT, onExpired);
        return () => window.removeEventListener(SESSION_EXPIRED_EVENT, onExpired);
    }, [navigate, queryClient]);

    if (!session) return <Navigate to={ROUTES.login} replace state={loginRedirectState(location.pathname)} />;
    return <>{children}</>;
}
