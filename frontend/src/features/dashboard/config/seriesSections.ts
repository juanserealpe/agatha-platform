import type { SectionKey } from "./sectionStyles";

export interface ThresholdBand {
    label: string;
    from: number;
    to?: number; // sin `to` = hasta el tope del eje
    color: string;
}

export interface SeriesChartConfig {
    title: string;
    seriesKeys: string[];
    thresholds?: ThresholdBand[];
    colors?: string[]; // un color por serie
    wide?: boolean;    // ocupa las dos columnas
}

export interface SeriesSectionConfig {
    charts: SeriesChartConfig[];
    variables: string[]; // lo que se pide al endpoint, derivado de las gráficas
}

const INDICADOR_THRESHOLDS: ThresholdBand[] = [
    // PENDIENTE: definir los valores reales con backend. Forma esperada:
    // { label: "Normal", from: 0, to: ???, color: "#10b981" },
    // { label: "Anómalo", from: ???, to: ???, color: "#f59e0b" },
    // { label: "Persistente", from: ???, to: ???, color: "#f97316" },
    // { label: "Crítico", from: ???, color: "#ef4444" },
];

function defineSection(charts: SeriesChartConfig[]): SeriesSectionConfig {
    return { charts, variables: [...new Set(charts.flatMap((c) => c.seriesKeys))] };
}

export const SERIES_SECTIONS: Record<SectionKey, SeriesSectionConfig> = {
    fuga: defineSection([
        { title: "Indicador I (24 h)", seriesKeys: ["indicador"], thresholds: INDICADOR_THRESHOLDS, colors: ["#8b5cf6"] },
        { title: "Densidad (24 h)", seriesKeys: ["densidad"], colors: ["#0d9488"] },
        { title: "Bandas de fuga (B1 vs. B2)", seriesKeys: ["b1", "b2"], colors: ["#3b82f6", "#f59e0b"], wide: true },
    ]),
    movimiento: defineSection([
        { title: "Pitch y Roll", seriesKeys: ["pitch", "roll"], colors: ["#3b82f6", "#f97316"], wide: true },
        { title: "Deriva", seriesKeys: ["deriva"], colors: ["#ef4444"] },
        { title: "Aceleración", seriesKeys: ["aceleracion"], colors: ["#8b5cf6"] },
    ]),
    ambiental: defineSection([
        { title: "Temperatura", seriesKeys: ["temperatura"], colors: ["#f97316"] },
        { title: "Humedad", seriesKeys: ["humedad"], colors: ["#0ea5e9"] },
    ]),
    energia: defineSection([
        { title: "Voltaje", seriesKeys: ["voltaje"], colors: ["#10b981"] },
        { title: "Corriente", seriesKeys: ["corriente"], colors: ["#eab308"] },
    ]),
};