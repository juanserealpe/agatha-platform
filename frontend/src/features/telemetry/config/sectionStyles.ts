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
        subtitle: "Indicador de fuga, densidad y bandas acústicas B1 y B2",
        badgeClass: "bg-amber-50 text-amber-600 border-amber-200",
    },
    movimiento: {
        icon: Compass,
        label: "Movimiento",
        subtitle: "Deriva y aceleración del sensor",
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
        subtitle: "Voltaje y corriente del sensor",
        badgeClass: "bg-violet-50 text-violet-600 border-violet-200",
    },
};