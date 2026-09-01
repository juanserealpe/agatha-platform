import {
    BatteryFull,
    BatteryMedium,
    BatteryWarning,
    SignalHigh,
    SignalMedium,
    SignalLow,
    SignalZero,
} from "lucide-react";
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

// senalCelular: 0-4 barras
export function getSignalIcon(senalCelular: number) {
    if (senalCelular >= 3) return SignalHigh;
    if (senalCelular === 2) return SignalMedium;
    if (senalCelular === 1) return SignalLow;
    return SignalZero;
}

export function getAlertVisual(nivel: NivelAlerta) {
    switch (nivel) {
        case "roja":
            return {
                dotClass: "bg-alerta-roja animate-pulse",
                ringClass: "border-alerta-roja",
                bgClass: "bg-red-50",
                clickable: true,
            };
        case "amarilla":
            return {
                dotClass: "bg-alerta-amarilla",
                ringClass: "border-alerta-amarilla",
                bgClass: "bg-amber-50",
                clickable: true,
            };
        default:
            return {
                dotClass: "bg-alerta-verde",
                ringClass: "border-alerta-verde",
                bgClass: "bg-emerald-50",
                clickable: true,
            };
    }
}

export function formatUltimaComunicacion(iso: string) {
    const diffMs = Date.now() - new Date(iso).getTime();
    const mins = Math.floor(diffMs / 60000);
    if (mins < 1) return "Hace instantes";
    if (mins < 60) return `Hace ${mins} min`;
    const hours = Math.floor(mins / 60);
    if (hours < 24) return `Hace ${hours} h`;
    return `Hace ${Math.floor(hours / 24)} d`;
}

export function getBannerVisual(nivel: NivelAlerta) {
    switch (nivel) {
        case "roja":
            return {
                borderClass: "border-2 border-alerta-roja",
                bgClass: "bg-red-50",
                labelColorClass: "text-alerta-roja",
                labelText: "ALERTA ROJA",
            };
        case "amarilla":
            return {
                borderClass: "border-2 border-alerta-amarilla",
                bgClass: "bg-amber-50",
                labelColorClass: "text-amber-600",
                labelText: "ALERTA AMARILLA",
            };
        default:
            return {
                borderClass: "border border-emerald-200",
                bgClass: "bg-emerald-50",
                labelColorClass: "text-emerald-600",
                labelText: "NORMAL",
            };
    }
}

export function getMarkerColor(nivel: NivelAlerta) {
    switch (nivel) {
        case "roja": return "#ef4444";
        case "amarilla": return "#f59e0b";
        default: return "#10b981";
    }
}