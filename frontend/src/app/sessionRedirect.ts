import { ROUTES } from "./routes";

interface LoginRedirectState {
    from: string;
}

/** Estado que lleva la redirección al login: la página que se pidió sin sesión. */
export function loginRedirectState(pathname: string): LoginRedirectState {
    return { from: pathname };
}

/**
 * A dónde ir tras el login: la página pedida si es una ruta interna de la app (nunca `/login` ni una URL
 * externa como `//otro.sitio`); si no, monitoreo.
 */
export function redirectAfterLogin(state: unknown): string {
    const from = typeof state === "object" && state !== null ? (state as Record<string, unknown>).from : undefined;
    if (typeof from !== "string" || !from.startsWith("/") || from.startsWith("//") || from === ROUTES.login) {
        return ROUTES.monitoring;
    }
    return from;
}
