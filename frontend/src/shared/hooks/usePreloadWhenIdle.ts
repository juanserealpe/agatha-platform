import { useEffect } from "react";

// Margen para no competir con la carga inicial de la pantalla antes de precargar.
const IDLE_PRELOAD_TIMEOUT_MS = 3000;

/**
 * Descarga un módulo diferido (lazy) cuando el navegador queda libre, para que al usarlo ya esté listo.
 * `load` debe ser estable (definido a nivel de módulo): import() ya evita descargas repetidas.
 */
export function usePreloadWhenIdle(load: () => Promise<unknown>) {
    useEffect(() => {
        if ("requestIdleCallback" in window) {
            const handle = window.requestIdleCallback(() => void load(), { timeout: IDLE_PRELOAD_TIMEOUT_MS });
            return () => window.cancelIdleCallback(handle);
        }
        const timer = setTimeout(() => void load(), IDLE_PRELOAD_TIMEOUT_MS);
        return () => clearTimeout(timer);
    }, [load]);
}
