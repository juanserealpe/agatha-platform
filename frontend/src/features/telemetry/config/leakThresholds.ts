import { SEVERITY_STYLES } from "@/features/alerts";
import type { LeakThresholds } from "@/services/dashboard/series.types";

// Plantilla beta del Rule Engine (backend: rule_engine/domain/entities.DefaultThresholdParams).
// Los umbrales son versionados por punto: cuando la serie del indicador traiga `thresholds`,
// se usan esos y esta plantilla queda solo como respaldo.
export const DEFAULT_LEAK_THRESHOLDS: LeakThresholds = { amarillo: 0.18, naranja: 0.25, rojo: 0.35 };

export type LeakLevel = "normal" | "amarillo" | "naranja" | "rojo";

interface LeakLevelStyle {
    label: string;
    badgeClass: string;
    /** Color de la franja y la línea de umbral (Recharts no usa clases de Tailwind). */
    hex: string;
}

// Misma escala que las alertas: el nivel que marca el indicador es el que verá el operador en la alerta.
export const LEAK_LEVEL_STYLES: Record<LeakLevel, LeakLevelStyle> = {
    normal: { label: "Normal", badgeClass: "bg-emerald-50 text-emerald-700 ring-1 ring-emerald-200", hex: "#10b981" },
    amarillo: { label: "Amarillo", badgeClass: SEVERITY_STYLES.yellow.badgeClass, hex: SEVERITY_STYLES.yellow.hex },
    naranja: { label: "Naranja", badgeClass: SEVERITY_STYLES.orange.badgeClass, hex: SEVERITY_STYLES.orange.hex },
    rojo: { label: "Rojo", badgeClass: SEVERITY_STYLES.red.badgeClass, hex: SEVERITY_STYLES.red.hex },
};
