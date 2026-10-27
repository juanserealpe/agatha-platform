import { extractPk, formatPk } from "@/features/sensors";

interface NamedDevice {
    id: number;
    name: string;
}

/** "PK34-470" → "Sensor PK 34+470", con el mismo formato de PK del mapa y los paneles. */
export function deviceDisplayName(name: string): string {
    const pkMeters = extractPk(name);
    return `Sensor ${pkMeters === null ? name : formatPk(pkMeters)}`;
}

/** Nombre del dispositivo para filtros y resúmenes; "Sensor <id>" si no está en la lista. */
export function deviceOptionLabel(id: number, devices: readonly NamedDevice[]): string {
    const device = devices.find((candidate) => candidate.id === id);
    return device ? deviceDisplayName(device.name) : `Sensor ${id}`;
}
