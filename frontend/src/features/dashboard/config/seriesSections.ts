//Qué se muestra: gráficas, variables y umbrales
export interface ThresholdBand {
    label: string;
    from: number;
    to?: number; // sin `to` = hasta el tope del eje
    color: string;
}

export interface SeriesChartConfig {
    title: string;
    seriesKeys: string[]; // series que comparten una misma gráfica
    thresholds?: ThresholdBand[];
}

export interface SeriesSectionConfig {
    label: string;
    charts: SeriesChartConfig[];
}

const INDICADOR_THRESHOLDS: ThresholdBand[] = [
    // PENDIENTE: definir los valores reales con backend. Forma esperada:
    // { label: "Normal", from: 0, to: ???, color: "#10b981" },
    // { label: "Anómalo", from: ???, to: ???, color: "#f59e0b" },
    // { label: "Persistente", from: ???, to: ???, color: "#f97316" },
    // { label: "Crítico", from: ???, color: "#ef4444" },
];

export const FUGA_SECTION: SeriesSectionConfig = {
    label: "Fuga",
    charts: [
        { title: "Indicador I (24 h)", seriesKeys: ["indicador"], thresholds: INDICADOR_THRESHOLDS },
        { title: "Densidad (24 h)", seriesKeys: ["densidad"] },
        { title: "Bandas de fuga (B1 vs. B2)", seriesKeys: ["b1", "b2"] },
    ],
};

// variables que se piden al endpoint, sin repetir
export const FUGA_VARIABLES = [...new Set(FUGA_SECTION.charts.flatMap((c) => c.seriesKeys))];