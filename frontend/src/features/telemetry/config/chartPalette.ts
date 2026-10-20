import type { Series } from "@/services/dashboard/series.types";

// El color de una serie dice de dónde viene el dato, no qué variable es (el título ya lo dice).
// Ninguno pertenece a la escala amarillo/naranja/rojo, reservada a los umbrales de alerta.
export const SOURCE_COLORS: Record<Series["source"], string> = {
    measurement: "#2a78d6", // lectura del sensor
    evaluation: "#6250d6", // calculado por el sistema
};

export const SOURCE_LABELS: Record<Series["source"], string> = {
    measurement: "Lectura del sensor (~15 min)",
    evaluation: "Calculado cada hora con las 24 h previas",
};

// Orden fijo (validado para daltonismo) para gráficas con varias series. Sin rojos ni amarillos.
export const MULTI_SERIES_COLORS = ["#2a78d6", "#1baf7a", "#6250d6", "#e87ba4"];

/** Tono de la cuadrícula, ejes y referencias neutras. */
export const CHART_INK = {
    grid: "#e2e8f0",
    axis: "#64748b",
    reference: "#475569",
};
