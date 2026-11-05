/**
 * PK (punto kilométrico): posición sobre el ducto expresada como "km+metros", p. ej. PK 34+470.
 * Internamente se trabaja en metros desde el inicio del tramo.
 */

const METERS_PER_KM = 1000;

// "PK34-470", "pk 34+470", "PK 34 + 470". El backend usa guion en los nombres de dispositivo.
const PK_IN_TEXT = /pk\s*(\d{1,3})\s*[+-]\s*(\d{3})(?!\d)/i;

// Con separador, el km es exacto y los metros se completan mientras se escribe: "34+", "34+4", "34-470", "34 470".
const PK_QUERY_WITH_SEPARATOR = /^(\d{1,3})[+\-. ](\d{0,3})$/;
// Solo dígitos: prefijo de los dígitos del PK, para filtrar mientras se escribe: "3", "344", "34470".
const PK_QUERY_DIGITS = /^\d{1,6}$/;

export type PkQuery =
    | { kind: "parts"; km: number; metersPrefix: string }
    | { kind: "digits"; digits: string };

/** Extrae el PK (en metros) de un texto como el nombre del dispositivo. */
export function extractPk(text: string): number | null {
    const match = PK_IN_TEXT.exec(text);
    if (!match) return null;
    return Number(match[1]) * METERS_PER_KM + Number(match[2]);
}

/** "PK 34+470" a partir de metros. */
export function formatPk(meters: number): string {
    const km = Math.floor(meters / METERS_PER_KM);
    const rest = meters % METERS_PER_KM;
    return `PK ${String(km).padStart(2, "0")}+${String(rest).padStart(3, "0")}`;
}

/** Interpreta lo que el usuario escribe en el buscador como un PK, parcial o completo. */
export function parsePkQuery(input: string): PkQuery | null {
    const value = input.trim().toLowerCase().replace(/^pk\s*/, "").replace(/\s*([+\-.])\s*/, "$1");
    if (!value) return null;

    if (PK_QUERY_DIGITS.test(value)) return { kind: "digits", digits: value };

    const withSeparator = PK_QUERY_WITH_SEPARATOR.exec(value);
    if (withSeparator) return { kind: "parts", km: Number(withSeparator[1]), metersPrefix: withSeparator[2] };

    return null;
}

/** Indica si un PK (en metros) coincide con una consulta parcial. */
export function pkMatchesQuery(pkMeters: number, query: PkQuery): boolean {
    const km = Math.floor(pkMeters / METERS_PER_KM);
    const meters = String(pkMeters % METERS_PER_KM).padStart(3, "0");

    if (query.kind === "parts") {
        return km === query.km && meters.startsWith(query.metersPrefix);
    }
    // Se acepta el km con o sin cero a la izquierda: PK 02+100 coincide con "2100" y con "02100".
    return [`${km}${meters}`, `${String(km).padStart(2, "0")}${meters}`].some((digits) =>
        digits.startsWith(query.digits)
    );
}
