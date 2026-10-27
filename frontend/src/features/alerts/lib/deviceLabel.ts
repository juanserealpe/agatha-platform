import { extractPk, formatPk } from "@/features/sensors";

interface NamedDevice {
    id: number;
    name: string;
}

/**
 * Solo el PK del nombre del dispositivo, con el formato del mapa y los paneles:
 * "Sensor MP-11556 PK34-470" → "PK 34+470". Sin PK reconocible, el nombre tal cual.
 */
export function devicePkLabel(name: string): string {
    const pkMeters = extractPk(name);
    return pkMeters === null ? name : formatPk(pkMeters);
}

/** "Sensor MP-11556 PK34-470" → "Sensor PK 34+470". */
export function deviceDisplayName(name: string): string {
    return `Sensor ${devicePkLabel(name)}`;
}

/** Nombre del dispositivo para filtros y resúmenes; "Sensor <id>" si no está en la lista. */
export function deviceOptionLabel(id: number, devices: readonly NamedDevice[]): string {
    const device = devices.find((candidate) => candidate.id === id);
    return device ? deviceDisplayName(device.name) : `Sensor ${id}`;
}
