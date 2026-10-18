import type { Device } from "@/services/dashboard/dashboard.types";
import { extractPk, parsePkQuery, pkMatchesQuery } from "./pk";

function normalize(text: string): string {
    return text.normalize("NFD").replace(/\p{Diacritic}/gu, "").toLowerCase().trim();
}

/** Coincide por nombre (sin tildes ni mayúsculas) o por PK en cualquier formato. */
export function matchesSensorQuery(device: Device, query: string): boolean {
    const normalizedQuery = normalize(query);
    if (!normalizedQuery) return true;

    if (normalize(device.name).includes(normalizedQuery)) return true;

    const pkQuery = parsePkQuery(query);
    const devicePk = extractPk(device.name);
    return pkQuery !== null && devicePk !== null && pkMatchesQuery(devicePk, pkQuery);
}

export function filterDevices(devices: Device[], query: string): Device[] {
    return devices.filter((device) => matchesSensorQuery(device, query));
}
