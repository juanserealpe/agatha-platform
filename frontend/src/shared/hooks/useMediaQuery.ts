import { useCallback, useSyncExternalStore } from "react";

/** true mientras la consulta de medios se cumple, p. ej. `(min-width: 640px)`. Sin `window` (pruebas), false. */
export function useMediaQuery(query: string): boolean {
    const subscribe = useCallback(
        (onChange: () => void) => {
            const list = window.matchMedia(query);
            list.addEventListener("change", onChange);
            return () => list.removeEventListener("change", onChange);
        },
        [query]
    );
    return useSyncExternalStore(
        subscribe,
        () => window.matchMedia(query).matches,
        () => false
    );
}
