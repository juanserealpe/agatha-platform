import { BatteryFull, BatteryMedium, BatteryWarning } from "lucide-react";
import type { NivelAlerta } from "@/services/types";

export function getBatteryVisual(bateria: number) {
    if (bateria > 60) {
        return { Icon: BatteryFull, colorClass: "text-alerta-verde" };
    }
    if (bateria >= 30) {
        return { Icon: BatteryMedium, colorClass: "text-alerta-amarilla" };
    }
    return { Icon: BatteryWarning, colorClass: "text-alerta-roja" };
}

export function getMarkerColor(nivel: NivelAlerta) {
    switch (nivel) {
        case "roja": return "#ef4444";
        case "amarilla": return "#f59e0b";
        default: return "#10b981";
    }
}
