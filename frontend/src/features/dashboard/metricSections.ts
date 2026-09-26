/* eslint-disable @typescript-eslint/no-unused-vars */
// features/dashboard/metricSections.ts
import type { MetricMeasurements } from "@/services/dashboard/measurement.types";

export type MetricSectionKey = "movimiento" | "acustica" | "ambiental" | "energia";

export interface ChartGroup {
    title: string;
    metricIds: number[]; // 1 o más metricId combinados en una sola gráfica
}

interface Section {
    label: string;
    groups: ChartGroup[];
}

export const METRIC_SECTIONS: Record<MetricSectionKey, Section> = {
    movimiento: {
        label: "Movimiento y orientación",
        groups: [
            { title: "Pitch (actual vs. base)", metricIds: [1, 15] },
            { title: "Roll (actual vs. base)", metricIds: [2, 16] },
            { title: "Deriva", metricIds: [19] },
            { title: "Magnitud promedio", metricIds: [8] },
            { title: "Actividad", metricIds: [7] },
            { title: "Actividad pico", metricIds: [9] },
            { title: "RMS (vibración)", metricIds: [6] },
        ],
    },
    acustica: {
        label: "Acústica y detección de fuga",
        groups: [
            { title: "Bandas de fuga (B1 vs. B2)", metricIds: [12, 13] },
            { title: "RMS acústico", metricIds: [17] },
            { title: "B0 (ruido de viento)", metricIds: [11] },
            { title: "B3 (banda de guarda)", metricIds: [14] },
            { title: "Razón R (fuga/viento)", metricIds: [10] },
            { title: "Planitud espectral", metricIds: [18] },
        ],
    },
    ambiental: {
        label: "Ambiental",
        groups: [
            { title: "Temperatura", metricIds: [20] },
            { title: "Humedad", metricIds: [21] },
        ],
    },
    energia: {
        label: "Energía",
        groups: [
            { title: "Batería", metricIds: [5] },
            { title: "Voltaje", metricIds: [3] },
            { title: "Corriente", metricIds: [4] },
        ],
    },
};