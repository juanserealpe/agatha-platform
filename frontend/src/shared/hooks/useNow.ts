import { useSyncExternalStore } from "react";

const TICK_MS = 30_000;

// Un único reloj compartido: todos los componentes que muestran tiempo relativo
// se actualizan juntos y no se crea un intervalo por instancia.
let now = Date.now();
let timer: ReturnType<typeof setInterval> | undefined;
const listeners = new Set<() => void>();

function subscribe(listener: () => void) {
    listeners.add(listener);
    if (!timer) {
        now = Date.now();
        timer = setInterval(() => {
            now = Date.now();
            listeners.forEach((notify) => notify());
        }, TICK_MS);
    }
    return () => {
        listeners.delete(listener);
        if (listeners.size === 0) {
            clearInterval(timer);
            timer = undefined;
        }
    };
}

function getSnapshot() {
    return now;
}

/** Marca de tiempo actual (ms) que se refresca cada 30 s. */
export function useNow(): number {
    return useSyncExternalStore(subscribe, getSnapshot);
}
