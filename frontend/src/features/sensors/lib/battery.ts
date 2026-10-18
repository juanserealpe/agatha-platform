import { Battery, BatteryFull, BatteryMedium, BatteryWarning, type LucideIcon } from "lucide-react";

// Provisional: los rangos definitivos dependen de la curva de voltaje del dispositivo.
const LOW_BATTERY_PERCENT = 30;
const HIGH_BATTERY_PERCENT = 80;

export interface BatteryVisual {
    Icon: LucideIcon;
    colorClass: string;
    label: string;
    isLow: boolean;
}

/** Verde si está alta, neutra en el rango medio y resaltada si está baja. */
export function getBatteryVisual(level: number | null): BatteryVisual {
    if (level === null) {
        return { Icon: Battery, colorClass: "text-gray-400", label: "Sin dato", isLow: false };
    }

    const label = `${Math.round(level)}%`;
    if (level < LOW_BATTERY_PERCENT) {
        return { Icon: BatteryWarning, colorClass: "text-red-600 font-semibold", label, isLow: true };
    }
    if (level < HIGH_BATTERY_PERCENT) {
        return { Icon: BatteryMedium, colorClass: "text-gray-600", label, isLow: false };
    }
    return { Icon: BatteryFull, colorClass: "text-emerald-600", label, isLow: false };
}
