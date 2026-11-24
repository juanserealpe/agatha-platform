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
    /** Deja que el eje baje de 0 aunque los datos sean positivos: evita que valores cercanos a 0 queden pegados al borde inferior. */
    allowBelowZero?: boolean;
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
// El voltaje llega en V (catálogo de series del backend): 0,1 V, no 100, que dejaba la línea plana sobre un eje 0–120.
const MIN_SPAN = { driftDeg: 0.1, accelG: 0.1, temperatureC: 2, humidityPct: 5, voltageV: 0.1, currentMa: 10 };

const TILT_HOW_TO_READ = [
    "En un sensor bien fijado la línea es prácticamente plana.",
    "Un escalón (cambio brusco que se mantiene) indica que el sensor o el tramo de tubería se movió.",
];
export const SERIES_SECTIONS: Record<SectionKey, SeriesSectionConfig> = {
    fuga: defineSection([
        {
            // Mismo nombre que usan los mensajes del catálogo del backend.
            title: "Índice de anomalía acústica I",
            seriesKeys: ["indicador"],
            // Puntaje de anomalía, no probabilidad: mismo mensaje que la nota del modal de alertas (ANOMALY_SCORE_NOTE).
            description:
                "Evolución de qué tan anómalo es el sonido frente a lo habitual en este punto. No es una probabilidad de fuga.",
            howToRead: [
                "Cerca de 0 el sonido es el habitual; entre más alto, más se aleja de lo normal y más atención merece.",
                "Las franjas amarilla, naranja y roja indican el nivel de alerta: mientras más arriba, mayor la urgencia.",
                "La alerta solo se activa si el valor se mantiene alto varias horas, no por un pico aislado.",
            ],
            leakThresholds: true,
            yScale: { min: 0, minTop: 0.4, max: 1 },
        },
        {
            title: "Densidad D",
            seriesKeys: ["densidad"],
            description: "Porcentaje del último día en que el sonido agudo estuvo más fuerte de lo normal.",
            howToRead: [
                `En condiciones normales ronda el ${EXPECTED_DENSITY_PCT} % (línea punteada).`,
                "Si se mantiene muy por encima, el sonido está más alto de lo habitual y eso hace subir el índice de anomalía acústica.",
            ],
            references: [{ label: `Normal ≈ ${EXPECTED_DENSITY_PCT} %`, value: EXPECTED_DENSITY_PCT }],
            yScale: { min: 0, minTop: 25, max: 100 },
        },
        {
            title: "Sonido agudo (B2)",
            seriesKeys: ["b2"],
            description: "Intensidad del sonido agudo (1200–2000 Hz) que capta el sensor. Es la señal principal para detectar fugas.",
            howToRead: [
                "Los picos cortos son normales (vehículos, maquinaria cercana).",
                "Preocupa cuando se mantiene alto durante horas.",
            ],
            wide: true,
        },
        {
            title: "Sonido grave (B1)",
            seriesKeys: ["b1"],
            description: "Intensidad del sonido grave (500–1200 Hz). Se compara con el agudo para distinguir una posible fuga de otros ruidos.",
            howToRead: [
                "En una fuga real el sonido grave y el agudo suben juntos.",
                "Si solo sube uno de los dos, probablemente no es una fuga.",
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
            yScale: { minSpan: MIN_SPAN.driftDeg },
        },
        {
            title: "Roll",
            seriesKeys: ["roll"],
            description: "Alabeo: inclinación lateral del sensor (°).",
            howToRead: TILT_HOW_TO_READ,
            yScale: { minSpan: MIN_SPAN.driftDeg },
        },
        {
            title: "Deriva",
            seriesKeys: ["deriva"],
            description: "Cuánto se ha girado el sensor respecto a su posición original (°). Cerca de 0 = no se ha movido.",
            howToRead: [
                "Si sube poco a poco, el sensor o el terreno se está desplazando lentamente.",
                "Un salto repentino suele ser un golpe o alguien trabajando en el sitio.",
            ],
            yScale: { minSpan: MIN_SPAN.driftDeg, allowBelowZero: true },
        },
        {
            title: "Aceleración",
            seriesKeys: ["aceleracion"],
            description: "Vibración que siente el sensor (g). En reposo marca cerca de 1 por la gravedad.",
            howToRead: ["Si se aleja de 1, el sensor recibió vibraciones o golpes."],
            yScale: { minSpan: MIN_SPAN.accelG },
        },
    ]),
    ambiental: defineSection([
        {
            title: "Temperatura",
            seriesKeys: ["temperatura"],
            description: "Qué tan caliente está el sensor (°C).",
            howToRead: ["Sube de día y baja de noche. Un cambio brusco fuera de ese patrón es poco habitual."],
            yScale: { minSpan: MIN_SPAN.temperatureC },
        },
        {
            title: "Humedad",
            seriesKeys: ["humedad"],
            description: "Cantidad de humedad en el aire alrededor del sensor (%).",
            howToRead: ["Sube con lluvia o condensación; ayuda a explicar cambios en el sonido."],
            yScale: { max: 100, minSpan: MIN_SPAN.humidityPct },
        },
    ]),
    energia: defineSection([
        {
            title: "Voltaje",
            seriesKeys: ["voltaje"],
            description: "Nivel de carga de la batería del sensor.",
            howToRead: ["Si baja de forma sostenida, la batería se está agotando y conviene revisarla."],
            yScale: { minSpan: MIN_SPAN.voltageV },
        },
        {
            title: "Corriente",
            seriesKeys: ["corriente"],
            description: "Cuánta energía está gastando el sensor en este momento.",
            howToRead: ["Los picos cortos son normales cuando el sensor transmite. Un consumo alto constante gasta la batería más rápido."],
            yScale: { minSpan: MIN_SPAN.currentMa },
        },
    ]),
};
