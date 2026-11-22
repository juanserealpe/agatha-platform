import { describe, expect, it } from "vitest";
import { computeTimeTicks, MIN_LABEL_SPACING_PX, pickAxisStep, type TimeTicksInput } from "./timeAxis";

const HOUR_MS = 3_600_000;
const DAY_MS = 24 * HOUR_MS;
const ms = (iso: string) => new Date(iso).getTime();
const DESKTOP_PX = 400;
const MOBILE_PX = 300;

function ticks(input: Partial<TimeTicksInput> & Pick<TimeTicksInput, "startMs" | "endMs">) {
    return computeTimeTicks({ widthPx: DESKTOP_PX, narrow: false, nowMs: null, ...input });
}

/** Distancia mínima en px entre etiquetas consecutivas. */
function minGapPx(list: ReturnType<typeof ticks>, widthPx: number): number {
    const gaps = list.slice(1).map((tick, i) => ((tick.offsetPct - list[i].offsetPct) / 100) * widthPx);
    return Math.min(...gaps);
}

describe("pickAxisStep", () => {
    it("elige el intervalo más corto que deja espacio a las etiquetas", () => {
        // 6 h en 400 px: 1 h = 66 px ≥ 64.
        expect(pickAxisStep(6 * HOUR_MS, 400, MIN_LABEL_SPACING_PX.wide)).toBe(HOUR_MS);
        // 2 d en 400 px: 6 h = 50 px no alcanza; 12 h = 100 px sí.
        expect(pickAxisStep(2 * DAY_MS, 400, MIN_LABEL_SPACING_PX.wide)).toBe(12 * HOUR_MS);
    });

    it("si ninguno alcanza, usa el de 7 días", () => {
        expect(pickAxisStep(365 * DAY_MS, 400, MIN_LABEL_SPACING_PX.wide)).toBe(7 * DAY_MS);
    });
});

describe("computeTimeTicks", () => {
    it("6 h: cada hora, en horas exactas de Colombia", () => {
        const list = ticks({ startMs: ms("2026-11-20T09:30:00-05:00"), endMs: ms("2026-11-20T15:30:00-05:00") });
        expect(list.map((t) => t.label)).toEqual(["10:00", "11:00", "12:00", "13:00", "14:00", "15:00"]);
        expect(list.every((t) => !t.isMajor)).toBe(true);
    });

    it("2 d: cada 12 h; la medianoche muestra el día", () => {
        const list = ticks({ startMs: ms("2026-11-18T22:00:00-05:00"), endMs: ms("2026-11-20T22:00:00-05:00") });
        expect(list.map((t) => t.label)).toEqual(["jue 19", "12:00", "vie 20", "12:00"]);
        expect(list.map((t) => t.isMajor)).toEqual([true, false, true, false]);
    });

    it("4 d: un día por marca y ninguna etiqueta se monta sobre otra", () => {
        const list = ticks({ startMs: ms("2026-11-17T14:00:00-05:00"), endMs: ms("2026-11-21T17:00:00-05:00") });
        expect(list.map((t) => t.label)).toEqual(["mié 18", "jue 19", "vie 20", "sáb 21"]);
        expect(minGapPx(list, DESKTOP_PX)).toBeGreaterThanOrEqual(MIN_LABEL_SPACING_PX.wide);
    });

    it("3 semanas: semanas desde el lunes con día y mes", () => {
        const list = ticks({ startMs: ms("2026-11-01T00:00:00-05:00"), endMs: ms("2026-11-22T00:00:00-05:00"), widthPx: 200 });
        expect(list.map((t) => t.label)).toEqual(["2 nov", "9 nov", "16 nov"]);
    });

    it("en celular el día va sin su nombre y caben más marcas", () => {
        const range = { startMs: ms("2026-11-17T14:00:00-05:00"), endMs: ms("2026-11-21T17:00:00-05:00") };
        const list = ticks({ ...range, widthPx: MOBILE_PX, narrow: true });
        expect(list.filter((t) => t.isMajor).map((t) => t.label)).toEqual(["18", "19", "20", "21"]);
        expect(minGapPx(list, MOBILE_PX)).toBeGreaterThanOrEqual(MIN_LABEL_SPACING_PX.narrow);
    });

    it("quita las marcas que chocarían con Ahora, contando que su etiqueta crece hacia la izquierda", () => {
        const startMs = ms("2026-11-20T09:30:00-05:00");
        const endMs = ms("2026-11-20T15:30:00-05:00");
        // 1 h = 66,7 px; 14:00 queda a 77,8 px de Ahora (15:10): menos que 64 + 24, así que se quita.
        const list = ticks({ startMs, endMs, nowMs: ms("2026-11-20T15:10:00-05:00") });
        expect(list.map((t) => t.label)).toEqual(["10:00", "11:00", "12:00", "13:00"]);
    });

    it("sin ancho medido no hay marcas", () => {
        expect(ticks({ startMs: 0, endMs: DAY_MS, widthPx: 0 })).toEqual([]);
    });
});
