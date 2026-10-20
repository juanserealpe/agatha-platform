import type { SectionKey } from "./sectionStyles";

export interface ThresholdBand {
    label: string;
    from: number;
    to?: number; // sin `to` = hasta el tope del eje
    color: string;
}

/** Línea de referencia neutra (no es un umbral de alerta). */
export interface ReferenceMark {
    label: string;
    value: number;
}

/** Límites del eje Y. Sin ellos el eje se ajusta solo a los datos. */
export interface YScale {
    min?: number;    // piso fijo
    minTop?: number; // el tope nunca baja de aquí (mantiene visibles umbrales y referencias)
    max?: number;    // techo fijo de la escala (p. ej. 1 para el indicador, 100 para un %)
    /** Rango mínimo visible, en unidades de la variable: por debajo de esto un cambio es ruido del sensor. */
    minSpan?: number;
}

export interface SeriesChartConfig {
    title: string;
    seriesKeys: string[];
    /** Qué es la variable, en una línea. Siempre visible. */
    description: string;
    /** Cómo interpretarla. Se despliega bajo demanda. */
    howToRead: string[];
    /** Dibuja las franjas amarillo/naranja/rojo del Rule Engine y el nivel actual. */
    leakThresholds?: boolean;
    references?: ReferenceMark[];
    yScale?: YScale;
    wide?: boolean; // ocupa las dos columnas
}

export interface SeriesSectionConfig {
    charts: SeriesChartConfig[];
    variables: string[]; // lo que se pide al endpoint, derivado de las gráficas
    /** Si existe, la sección no consulta el endpoint y muestra este motivo. */
    unavailableReason?: string;
}

// θ_D es el P90 de B2 en la línea base: en comportamiento normal, 1 de cada 10 lecturas lo supera.
const EXPECTED_DENSITY_PCT = 10;

function defineSection(charts: SeriesChartConfig[], unavailableReason?: string): SeriesSectionConfig {
    return { charts, variables: [...new Set(charts.flatMap((c) => c.seriesKeys))], unavailableReason };
}

// Rangos mínimos del eje: propuesta inicial del frontend, a validar con quien conoce el sensor.
const MIN_SPAN = { tiltDeg: 2, driftDeg: 1, accelG: 0.1, temperatureC: 2, humidityPct: 5 };

const TILT_HOW_TO_READ = [
    "En un sensor bien fijado la línea es prácticamente plana.",
    "Un escalón (cambio brusco que se mantiene) indica que el sensor o el tramo de tubería se movió.",
];

export const SERIES_SECTIONS: Record<SectionKey, SeriesSectionConfig> = {
    fuga: defineSection([
        {
            title: "Indicador de fuga I",
            seriesKeys: ["indicador"],
            description: "Probabilidad combinada de fuga (0 a 1). Se calcula cada hora con las últimas 24 h.",
            howToRead: [
                "Bajo 0,18 el comportamiento es normal. Las franjas marcan desde qué valor el indicador entra en amarillo, naranja o rojo.",
                "Una sola hora sobre la franja no basta: la alerta se abre con 2 horas seguidas en amarillo o más. Si se mantiene 12 h en amarillo sube a naranja, y 6 h en naranja sube a roja.",
                "Para cerrarse, el indicador debe bajar de 0,16 durante 2 horas seguidas.",
                "Combina cuatro señales de B2 frente a la línea base del punto: cuánto dura la racha elevada, la densidad, el exceso sobre lo normal y la coherencia entre B1, B2 y planitud. Sin una racha sostenida el indicador se mantiene bajo aunque haya picos aislados.",
            ],
            leakThresholds: true,
            yScale: { min: 0, minTop: 0.4, max: 1 },
        },
        {
            title: "Densidad D",
            seriesKeys: ["densidad"],
            description: "% de lecturas de las últimas 24 h en que B2 superó su percentil 90 habitual (P90).",
            howToRead: [
                "El P90 sale de la línea base del punto: el valor de B2 que solo se supera el 10 % del tiempo en condiciones normales.",
                `Por eso, sin anomalías, la densidad ronda el ${EXPECTED_DENSITY_PCT} % (línea punteada).`,
                "Si se mantiene muy por encima, la banda alta está elevada más seguido de lo habitual: es una de las señales que empujan el indicador.",
            ],
            references: [{ label: `Esperado ≈ ${EXPECTED_DENSITY_PCT} %`, value: EXPECTED_DENSITY_PCT }],
            yScale: { min: 0, minTop: 25, max: 100 },
        },
        {
            title: "Banda B2 · 1200–2000 Hz",
            seriesKeys: ["b2"],
            description: "Nivel de la señal acústica en la banda alta. Es la que alimenta el indicador y la densidad.",
            howToRead: [
                "Los picos aislados son normales (vehículos, maquinaria cercana).",
                "Lo que preocupa es una elevación sostenida durante horas, sobre todo si B1 sube al mismo tiempo.",
                "Pasa el cursor: la línea vertical se sincroniza con las demás gráficas para comparar el mismo instante.",
            ],
            wide: true,
        },
        {
            title: "Banda B1 · 500–1200 Hz",
            seriesKeys: ["b1"],
            description: "Nivel de la señal acústica en la banda baja. Se compara con B2 para medir coherencia.",
            howToRead: [
                "En una fuga real B1 y B2 tienden a moverse juntas; si solo una de las dos sube, el indicador pesa menos ese evento.",
                "Su escala es distinta a la de B2, por eso va en una gráfica aparte.",
            ],
            wide: true,
        },
    ]),
    movimiento: defineSection([
        {
            title: "Pitch",
            seriesKeys: ["pitch"],
            description: "Cabeceo: inclinación del sensor hacia adelante o atrás (°).",
            howToRead: TILT_HOW_TO_READ,
            yScale: { minSpan: MIN_SPAN.tiltDeg },
        },
        {
            title: "Roll",
            seriesKeys: ["roll"],
            description: "Alabeo: inclinación lateral del sensor (°).",
            howToRead: TILT_HOW_TO_READ,
            yScale: { minSpan: MIN_SPAN.tiltDeg },
        },
        {
            title: "Deriva",
            seriesKeys: ["deriva"],
            description: "Desviación de la orientación del sensor (°). Cerca de 0 = sin movimiento.",
            howToRead: [
                "Valores que crecen poco a poco indican un desplazamiento lento del terreno o del soporte.",
                "Un salto aislado suele ser un golpe o una intervención en sitio.",
            ],
            yScale: { min: 0, minSpan: MIN_SPAN.driftDeg },
        },
        {
            title: "Aceleración",
            seriesKeys: ["aceleracion"],
            description: "Magnitud promedio de la aceleración medida (g). En reposo ronda 1 g por la gravedad.",
            howToRead: [
                "Desviaciones respecto a ~1 g indican vibración o impactos sobre el sensor.",
            ],
            yScale: { minSpan: MIN_SPAN.accelG },
        },
    ]),
    ambiental: defineSection([
        {
            title: "Temperatura",
            seriesKeys: ["temperatura"],
            description: "Temperatura medida en el sensor (°C).",
            howToRead: ["Sigue el ciclo día/noche. Cambios bruscos fuera de ese patrón pueden afectar las lecturas acústicas."],
            yScale: { minSpan: MIN_SPAN.temperatureC },
        },
        {
            title: "Humedad",
            seriesKeys: ["humedad"],
            description: "Humedad relativa en el sensor (%).",
            howToRead: ["Útil para descartar lluvia o condensación como causa de cambios en la señal."],
            yScale: { max: 100, minSpan: MIN_SPAN.humidityPct },
        },
    ]),
    energia: defineSection(
        [
            { title: "Voltaje", seriesKeys: ["voltaje"], description: "Voltaje de alimentación del sensor.", howToRead: [] },
            { title: "Corriente", seriesKeys: ["corriente"], description: "Corriente consumida por el sensor.", howToRead: [] },
        ],
        // El catálogo de /api/dashboard/series no incluye estas variables: pedirlas responde 400.
        "Las gráficas de energía aún no están disponibles: el servidor todavía no entrega voltaje ni corriente."
    ),
};
