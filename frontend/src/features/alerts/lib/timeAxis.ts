import { formatColombiaClock, formatColombiaDayMonth, formatColombiaWeekday } from "@/shared/lib/formatDate";

/**
 * Eje de la franja de tiempo (ajuste de la etapa 4, 21-nov): marcas a intervalos regulares, NO en los eventos de
 * las alertas. Las fechas exactas de cada alerta van en su fila. Así el eje no se amontona con varias alertas.
 */

const HOUR_MS = 3_600_000;
const DAY_MS = 24 * HOUR_MS;
const WEEK_MS = 7 * DAY_MS;
// Colombia no tiene horario de verano: UTC-5 fijo. Las marcas caen en horas y medianoches de Colombia.
const COLOMBIA_OFFSET_MS = -5 * HOUR_MS;
// El 1-ene-1970 fue jueves: el primer lunes de la época está 4 días después. Las semanas empiezan en lunes.
const EPOCH_TO_MONDAY_MS = 4 * DAY_MS;

/** Intervalos posibles, del más corto al más largo. Se usa el más corto que deje espacio a las etiquetas. */
export const AXIS_STEPS_MS = [HOUR_MS, 2 * HOUR_MS, 3 * HOUR_MS, 6 * HOUR_MS, 12 * HOUR_MS, DAY_MS, 2 * DAY_MS, WEEK_MS] as const;

/** Separación mínima entre etiquetas: "jue 19" en escritorio; en celular, solo "19". */
export const MIN_LABEL_SPACING_PX = { wide: 64, narrow: 44 } as const;
// "Ahora" cae cerca del borde derecho y su etiqueta se alinea a la derecha: ocupa ~40 px HACIA LA IZQUIERDA de su
// línea. Las marcas a su izquierda necesitan ese ancho extra para no quedar pegadas ("21 Ahora").
const NOW_LABEL_EXTRA_PX = 24;

export interface AxisTick {
    atMs: number;
    /** Posición en % del eje. */
    offsetPct: number;
    label: string;
    /** Medianoche de Colombia: la etiqueta es el día, no la hora. */
    isMajor: boolean;
}

export interface TimeTicksInput {
    /** Dominio visible del eje (con su margen). */
    startMs: number;
    endMs: number;
    /** Ancho real de la pista; sin medir (0) no hay marcas. */
    widthPx: number;
    /** Pantalla angosta: más marcas por ancho y el día sin su nombre. */
    narrow: boolean;
    /** "Ahora" si alguna alerta sigue en curso: se quitan las marcas que chocarían con su etiqueta. */
    nowMs: number | null;
}

/** El intervalo más corto que separa las etiquetas al menos `minLabelPx`; el más largo si ninguno alcanza. */
export function pickAxisStep(spanMs: number, widthPx: number, minLabelPx: number): number {
    return AXIS_STEPS_MS.find((step) => (step / spanMs) * widthPx >= minLabelPx) ?? AXIS_STEPS_MS[AXIS_STEPS_MS.length - 1];
}

/** Primera marca en o después de `startMs`, alineada a horas, medianoches o lunes de Colombia. */
function firstTickMs(startMs: number, stepMs: number): number {
    const anchor = stepMs === WEEK_MS ? EPOCH_TO_MONDAY_MS : 0;
    const local = startMs + COLOMBIA_OFFSET_MS - anchor;
    return Math.ceil(local / stepMs) * stepMs + anchor - COLOMBIA_OFFSET_MS;
}

function isColombiaMidnight(ms: number): boolean {
    return (ms + COLOMBIA_OFFSET_MS) % DAY_MS === 0;
}

function tickLabel(atMs: number, stepMs: number, isMajor: boolean, narrow: boolean): string {
    if (stepMs === WEEK_MS) return formatColombiaDayMonth(atMs);
    return isMajor ? formatColombiaWeekday(atMs, !narrow) : formatColombiaClock(atMs);
}

/** Marcas regulares del eje, sin las que chocarían con la etiqueta de "Ahora". */
export function computeTimeTicks({ startMs, endMs, widthPx, narrow, nowMs }: TimeTicksInput): AxisTick[] {
    const spanMs = endMs - startMs;
    if (widthPx <= 0 || spanMs <= 0) return [];
    const minLabelPx = narrow ? MIN_LABEL_SPACING_PX.narrow : MIN_LABEL_SPACING_PX.wide;
    const stepMs = pickAxisStep(spanMs, widthPx, minLabelPx);
    const toPx = (ms: number) => ((ms - startMs) / spanMs) * widthPx;
    const nowPx = nowMs === null ? null : toPx(nowMs);

    const ticks: AxisTick[] = [];
    for (let atMs = firstTickMs(startMs, stepMs); atMs <= endMs; atMs += stepMs) {
        if (nowPx !== null && Math.abs(toPx(atMs) - nowPx) < minLabelPx + NOW_LABEL_EXTRA_PX) continue;
        const isMajor = isColombiaMidnight(atMs);
        ticks.push({ atMs, offsetPct: (toPx(atMs) / widthPx) * 100, label: tickLabel(atMs, stepMs, isMajor, narrow), isMajor });
    }
    return ticks;
}
