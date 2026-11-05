import type { AlertLevel } from "@/services/alertManagement";
import type { Series, SeriesPoint, SeriesResponse } from "@/services/dashboard/series.types";
import { ApiError } from "@/services/http/apiError";
import type { LevelInterval } from "./alertStore";
import { INDICATOR_BY_LEVEL } from "./demoData";

// Series simuladas con la forma de GET /api/dashboard/series. Son deterministas (el mismo instante da siempre el
// mismo valor, así recargar no cambia la gráfica) y siguen los tramos de nivel de las alertas del punto: B2, B1,
// densidad e indicador suben juntos durante la alerta, como en una fuga real, y el resto queda en valores normales.

type Level = AlertLevel | "NORMAL";

interface SeriesDefinition {
    key: string;
    name: string;
    unit: string;
    source: Series["source"];
    value(context: SampleContext): number;
}

interface SampleContext {
    time: number;
    level: Level;
    /** Nivel una hora antes: suaviza el cambio para que no sea un escalón perfecto. */
    previousLevel: Level;
    /** Ruido determinista en [-1, 1]. */
    noise(salt: string): number;
}

const MINUTE_MS = 60 * 1000;
const HOUR_MS = 60 * MINUTE_MS;
// La telemetría llega cada 15 min; Processing evalúa por hora.
const MEASUREMENT_STEP_MS = 15 * MINUTE_MS;
const EVALUATION_STEP_MS = HOUR_MS;
const COLOMBIA_OFFSET_HOURS = -5;

// Cuánto sube cada señal acústica respecto a su nivel normal.
const B2_FACTOR: Record<Level, number> = { NORMAL: 1, AMARILLO: 1.8, NARANJA: 2.4, ROJO: 3 };
const B1_FACTOR: Record<Level, number> = { NORMAL: 1, AMARILLO: 1.4, NARANJA: 1.7, ROJO: 2 };
const DENSITY_PCT: Record<Level, number> = { NORMAL: 10, AMARILLO: 28, NARANJA: 42, ROJO: 58 };

const B2_BASE = 40;
const B1_BASE = 55;
/** Fracción de lecturas normales con un pico corto (vehículos, maquinaria cercana). */
const SPIKE_RATE = 0.04;

function blend(context: SampleContext, byLevel: Record<Level, number>): number {
    return (byLevel[context.level] + byLevel[context.previousLevel]) / 2;
}

/** Ciclo del día en hora de Colombia: 1 a las 14:00, -1 a las 02:00. */
function dayCycle(time: number): number {
    const hour = ((time / HOUR_MS + COLOMBIA_OFFSET_HOURS) % 24 + 24) % 24;
    return Math.cos(((hour - 14) / 24) * 2 * Math.PI);
}

function round(value: number, decimals: number): number {
    const factor = 10 ** decimals;
    return Math.round(value * factor) / factor;
}

// Mismo orden, claves, nombres y unidades que el catálogo del backend (dashboard/domain/entities/series_catalog.go).
const SERIES_CATALOG: SeriesDefinition[] = [
    {
        key: "b1",
        name: "B1 fuga 500-1200Hz",
        unit: "-",
        source: "measurement",
        value: (c) => round(B1_BASE * blend(c, B1_FACTOR) * (1 + 0.12 * c.noise("b1")), 2),
    },
    {
        key: "b2",
        name: "B2 fuga 1200-2000Hz",
        unit: "-",
        source: "measurement",
        value: (c) => {
            const spike = c.level === "NORMAL" && c.noise("b2-spike") > 1 - 2 * SPIKE_RATE ? 1.6 : 1;
            return round(B2_BASE * blend(c, B2_FACTOR) * spike * (1 + 0.12 * c.noise("b2")), 2);
        },
    },
    {
        key: "densidad",
        name: "Densidad (D, 24 h)",
        unit: "%",
        source: "evaluation",
        value: (c) => round(Math.max(0, blend(c, DENSITY_PCT) + 3 * c.noise("densidad")), 1),
    },
    {
        key: "indicador",
        name: "Indicador I (24 h)",
        unit: "-",
        source: "evaluation",
        // El ruido es menor que la mitad de la franja más angosta: el valor nunca cae en el nivel vecino.
        value: (c) => round(Math.max(0, blend(c, INDICATOR_BY_LEVEL) + 0.015 * c.noise("indicador")), 3),
    },
    {
        key: "temperatura",
        name: "Temperatura",
        unit: "°C",
        source: "measurement",
        value: (c) => round(22 + 5 * dayCycle(c.time) + 0.4 * c.noise("temperatura"), 1),
    },
    {
        key: "humedad",
        name: "Humedad",
        unit: "%",
        source: "measurement",
        value: (c) => round(Math.min(100, 78 - 12 * dayCycle(c.time) + 2 * c.noise("humedad")), 1),
    },
    {
        key: "deriva",
        name: "Deriva",
        unit: "°",
        source: "measurement",
        value: (c) => round(0.03 + 0.005 * c.noise("deriva"), 3),
    },
    {
        key: "pitch",
        name: "Pitch",
        unit: "°",
        source: "measurement",
        value: (c) => round(1.42 + 0.01 * c.noise("pitch"), 3),
    },
    {
        key: "roll",
        name: "Roll",
        unit: "°",
        source: "measurement",
        value: (c) => round(-0.86 + 0.01 * c.noise("roll"), 3),
    },
    {
        key: "aceleracion",
        name: "Aceleración (magnitud promedio)",
        unit: "g",
        source: "measurement",
        value: (c) => round(1 + 0.01 * c.noise("aceleracion"), 3),
    },
    {
        key: "voltaje",
        name: "Voltaje",
        unit: "V",
        source: "measurement",
        value: (c) => round(3.95 + 0.01 * c.noise("voltaje"), 3),
    },
    {
        key: "corriente",
        name: "Corriente",
        unit: "mA",
        source: "measurement",
        // Picos cortos cuando el sensor transmite.
        value: (c) => round(c.noise("corriente-tx") > 0.8 ? 45 + 3 * c.noise("corriente") : 11 + c.noise("corriente"), 1),
    },
];

/** Hash entero de un texto (FNV-1a de 32 bits). */
function hash(text: string): number {
    let value = 0x811c9dc5;
    for (let i = 0; i < text.length; i++) {
        value ^= text.charCodeAt(i);
        value = Math.imul(value, 0x01000193);
    }
    return value >>> 0;
}

function levelAt(intervals: readonly LevelInterval[], time: number): Level {
    const interval = intervals.find(
        (candidate) => candidate.from.getTime() <= time && (candidate.to === null || time < candidate.to.getTime())
    );
    return interval?.level ?? "NORMAL";
}

function samples(definition: SeriesDefinition, deviceId: number, from: number, to: number, intervals: readonly LevelInterval[]) {
    const step = definition.source === "evaluation" ? EVALUATION_STEP_MS : MEASUREMENT_STEP_MS;
    const data: SeriesPoint[] = [];
    for (let time = Math.ceil(from / step) * step; time <= to; time += step) {
        const context: SampleContext = {
            time,
            level: levelAt(intervals, time),
            previousLevel: levelAt(intervals, time - HOUR_MS),
            noise: (salt) => (hash(`${deviceId}:${salt}:${time}`) / 0xffffffff) * 2 - 1,
        };
        data.push({ timestamp: new Date(time).toISOString(), value: definition.value(context) });
    }
    return data;
}

export interface DemoSeriesRequest {
    deviceId: number;
    startDate: string;
    endDate: string;
    variables: string[];
    /** Tramos de nivel de las alertas del punto; vacío = todo normal. */
    intervals: readonly LevelInterval[];
    now: Date;
}

/** Igual que el backend: claves sin repetir, en el orden del catálogo; vacío = todas; una desconocida = 400. */
export function generateSeries({ deviceId, startDate, endDate, variables, intervals, now }: DemoSeriesRequest): SeriesResponse {
    const keys = variables.map((key) => key.trim().toLowerCase()).filter(Boolean);
    const unknown = keys.find((key) => !SERIES_CATALOG.some((definition) => definition.key === key));
    if (unknown !== undefined) {
        throw new ApiError({ status: 400, code: "INVALID_INPUT", message: `variable desconocida: ${unknown}` });
    }
    const from = Date.parse(startDate);
    // Sin datos del futuro: el rango se corta en el instante actual.
    const to = Math.min(Date.parse(endDate), now.getTime());
    const definitions = keys.length === 0 ? SERIES_CATALOG : SERIES_CATALOG.filter((definition) => keys.includes(definition.key));
    return {
        deviceId,
        startDate,
        endDate,
        series: definitions.map((definition) => ({
            key: definition.key,
            name: definition.name,
            unit: definition.unit,
            source: definition.source,
            data: samples(definition, deviceId, from, to, intervals),
        })),
    };
}
