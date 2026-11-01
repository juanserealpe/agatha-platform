/**
 * Modo de la app, decidido una sola vez al cargar la página: con una ruta bajo `/demo` arranca la demo
 * (datos simulados, sin backend); con cualquier otra, la app real. No se cambia de modo en caliente: para
 * salir de la demo se recarga la página en `/`.
 *
 * VITE_ENABLE_DEMO=false apaga la ruta sin tocar código: `/demo` cae en la redirección a monitoreo.
 */
export const DEMO_ENABLED = import.meta.env.VITE_ENABLE_DEMO !== "false";

export const DEMO_BASENAME = "/demo";

/** `/demo` o una ruta debajo (`/demo/monitoreo`); `/demostracion` no cuenta. */
export function isDemoPath(pathname: string): boolean {
    return pathname === DEMO_BASENAME || pathname.startsWith(`${DEMO_BASENAME}/`);
}
