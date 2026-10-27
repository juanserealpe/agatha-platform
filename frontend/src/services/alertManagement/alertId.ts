import type { AlertId } from "./alertManagement.types";

/** Marca como ID de alerta un número que ya se sabe que lo es (p. ej. leído de la URL). */
export function toAlertId(value: number): AlertId {
    return value as AlertId;
}

/** ID de alerta desde texto (p. ej. `?alerta=123`); null si no es un entero positivo. */
export function parseAlertId(raw: string | null): AlertId | null {
    if (raw === null || !/^\d+$/.test(raw.trim())) return null;
    const value = Number(raw);
    return Number.isSafeInteger(value) && value > 0 ? toAlertId(value) : null;
}
