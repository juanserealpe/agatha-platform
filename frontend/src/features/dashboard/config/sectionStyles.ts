import { AudioWaveform, Compass, Thermometer, BatteryCharging, type LucideIcon } from "lucide-react";

export type SectionKey = "fuga" | "movimiento" | "ambiental" | "energia";

export const SECTION_ORDER: SectionKey[] = ["fuga", "movimiento", "ambiental", "energia"];

interface SectionStyle {
    icon: LucideIcon;
    label: string;
    subtitle: string;
    badgeClass: string;
}

export const SECTION_STYLES: Record<SectionKey, SectionStyle> = {
    fuga: {
        icon: AudioWaveform,
        label: "Fuga",
        subtitle: "Análisis acústico espectral y detección por sub-bandas",
        badgeClass: "bg-amber-50 text-amber-600 border-amber-200",
    },
    movimiento: {
        icon: Compass,
        label: "Movimiento",
        subtitle: "Orientación, deriva y actividad del sensor",
        badgeClass: "bg-blue-50 text-blue-600 border-blue-200",
    },
    ambiental: {
        icon: Thermometer,
        label: "Ambiente",
        subtitle: "Temperatura y humedad del sensor y del entorno",
        badgeClass: "bg-emerald-50 text-emerald-600 border-emerald-200",
    },
    energia: {
        icon: BatteryCharging,
        label: "Energía del sensor",
        subtitle: "Batería, voltaje y corriente",
        badgeClass: "bg-violet-50 text-violet-600 border-violet-200",
    },
};