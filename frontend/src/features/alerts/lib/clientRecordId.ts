export interface ClientRecordIdTracker {
    /** UUID del intento: el mismo si se reintenta exactamente el mismo guardado tras un fallo. */
    idFor(payload: unknown): string | undefined;
    /** Tras un guardado exitoso, el siguiente intento estrena UUID. */
    reset(): void;
}

function defaultGenerate(): string | undefined {
    // randomUUID solo existe en contextos seguros (HTTPS o localhost). Sin él se omite el campo,
    // que es opcional: se pierde la protección de reintentos, no el guardado.
    return typeof crypto !== "undefined" && typeof crypto.randomUUID === "function" ? crypto.randomUUID() : undefined;
}

/**
 * Reintentar un guardado que falló (p. ej. se cortó la red después de que el backend lo recibió) reutiliza su
 * `client_record_id`, así el backend responde ALREADY_RECEIVED en vez de duplicarlo. Si el usuario cambia lo
 * que envía, es otro guardado y lleva otro UUID.
 */
export function createClientRecordIdTracker(generate: () => string | undefined = defaultGenerate): ClientRecordIdTracker {
    let last: { key: string; id: string | undefined } | null = null;

    return {
        idFor(payload) {
            const key = JSON.stringify(payload);
            if (last?.key !== key) last = { key, id: generate() };
            return last.id;
        },
        reset() {
            last = null;
        },
    };
}
