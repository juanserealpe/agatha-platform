import { describe, expect, it } from "vitest";
import {
    buildStripCursor,
    buildTimeStrip,
    compactStripHeightPx,
    markerTooltip,
    pieceAccessibleLabel,
    pieceTooltip,
    placeAxisLabels,
    tickLabel,
    tickLabelLines,
    type StripIndicatorInput,
} from "./timeStrip";

const at = (hhmm: string, day = 24) => `2026-08-${day}T${hhmm}:00-05:00`;
const ms = (hhmm: string, day = 24) => new Date(at(hhmm, day)).getTime();

const NOW = ms("15:00");

// Amarillo 05:00–08:00, Naranja 08:00–10:00, Rojo 10:00–en curso: 10 h de eje.
const activeLeak: StripIndicatorInput = {
    id: 1,
    event: "FUGA",
    endedAt: null,
    levels: [
        { seq: 1, level: "AMARILLO", opened_by: "OPEN", started_at: at("05:00"), ended_at: at("08:00") },
        { seq: 2, level: "NARANJA", opened_by: "UP", started_at: at("08:00"), ended_at: at("10:00") },
        { seq: 3, level: "ROJO", opened_by: "UP", started_at: at("10:00"), ended_at: null },
    ],
};

describe("buildTimeStrip", () => {
    it("sin tramos no hay franja", () => {
        expect(buildTimeStrip([{ ...activeLeak, levels: [] }], NOW)).toBeNull();
    });

    it("los tramos son proporcionales a la duración y llegan hasta ahora (CA4)", () => {
        const strip = buildTimeStrip([activeLeak], NOW);
        expect(strip?.active).toBe(true);
        const pieces = strip!.bars[0].pieces;
        expect(pieces.map((p) => [p.kind, p.level, p.offsetPct, p.widthPct])).toEqual([
            ["level", "AMARILLO", 0, 30],
            ["level", "NARANJA", 30, 20],
            ["level", "ROJO", 50, 50],
        ]);
        expect(pieces[2].reachesNow).toBe(true);
    });

    it("un círculo por cambio de nivel más el inicio, y uno hueco en ahora (CA4)", () => {
        const markers = buildTimeStrip([activeLeak], NOW)!.bars[0].markers;
        expect(markers.map((m) => [m.kind, m.level, m.from, m.offsetPct])).toEqual([
            ["change", "AMARILLO", null, 0],
            ["change", "NARANJA", "AMARILLO", 30],
            ["change", "ROJO", "NARANJA", 50],
            ["now", "ROJO", null, 100],
        ]);
    });

    it("tramos contiguos del mismo nivel no repiten círculo", () => {
        const strip = buildTimeStrip(
            [
                {
                    ...activeLeak,
                    levels: [
                        { seq: 1, level: "AMARILLO", opened_by: "OPEN", started_at: at("05:00"), ended_at: at("08:00") },
                        { seq: 2, level: "AMARILLO", opened_by: "DOWN", started_at: at("08:00"), ended_at: null },
                    ],
                },
            ],
            NOW
        );
        expect(strip!.bars[0].markers.filter((m) => m.kind === "change")).toHaveLength(1);
    });

    it("finalizada: sin círculo hueco; cierra con un círculo del color de la última fase y su fecha en el eje", () => {
        const ended: StripIndicatorInput = {
            id: 2,
            event: "FUGA",
            endedAt: at("09:00"),
            levels: [{ seq: 1, level: "AMARILLO", opened_by: "OPEN", started_at: at("05:00"), ended_at: at("09:00") }],
        };
        const strip = buildTimeStrip([ended], NOW)!;
        expect(strip.active).toBe(false);
        expect(strip.endMs).toBe(ms("09:00"));
        expect(strip.bars[0].markers.some((m) => m.kind === "now")).toBe(false);
        expect(strip.bars[0].markers.at(-1)).toMatchObject({ kind: "end", level: "AMARILLO", offsetPct: 100 });
        expect(markerTooltip(strip.bars[0].markers.at(-1)!)).toBe("24/08 09:00 · Finalizada · Amarillo");
        expect(strip.ticks.map(tickLabel)).toEqual(["24/08 05:00", "24/08 09:00"]);
    });

    it("el hueco de una reactivación es gris y el regreso lleva círculo", () => {
        const reactivated: StripIndicatorInput = {
            id: 3,
            event: "FUGA",
            endedAt: at("15:00"),
            levels: [
                { seq: 1, level: "AMARILLO", opened_by: "OPEN", started_at: at("05:00"), ended_at: at("07:00") },
                { seq: 2, level: "AMARILLO", opened_by: "REACTIVATION", started_at: at("13:00"), ended_at: at("15:00") },
            ],
        };
        const bar = buildTimeStrip([reactivated], NOW)!.bars[0];
        expect(bar.pieces.map((p) => [p.kind, p.offsetPct, p.widthPct])).toEqual([
            ["level", 0, 20],
            ["idle", 20, 60],
            ["level", 80, 20],
        ]);
        expect(bar.markers.map((m) => [m.kind, m.from, m.openedBy, m.offsetPct])).toEqual([
            ["change", null, "OPEN", 0],
            ["change", null, "REACTIVATION", 80],
            ["end", null, null, 100],
        ]);
    });

    it("varias barras comparten el eje; la que empieza después tiene gris antes", () => {
        const motion: StripIndicatorInput = {
            id: 4,
            event: "MOVIMIENTO",
            endedAt: null,
            levels: [{ seq: 1, level: "AMARILLO", opened_by: "OPEN", started_at: at("10:00"), ended_at: null }],
        };
        const strip = buildTimeStrip([activeLeak, motion], NOW)!;
        expect(strip.startMs).toBe(ms("05:00"));
        expect(strip.bars[1].pieces.map((p) => [p.kind, p.offsetPct, p.widthPct])).toEqual([
            ["idle", 0, 50],
            ["level", 50, 50],
        ]);
    });

    it("el eje solo marca los cambios de nivel y Ahora", () => {
        const ticks = buildTimeStrip([activeLeak], NOW)!.ticks;
        expect(ticks.map((t) => t.offsetPct)).toEqual([0, 30, 50, 100]);
        expect(ticks.map(tickLabel)).toEqual(["24/08 05:00", "24/08 08:00", "24/08 10:00", "Ahora"]);
        expect(ticks.map(tickLabelLines)).toEqual([["24/08", "05:00"], ["24/08", "08:00"], ["24/08", "10:00"], ["Ahora"]]);
    });

    it("barras que cambian a la misma hora comparten la marca", () => {
        const motion: StripIndicatorInput = {
            id: 4,
            event: "MOVIMIENTO",
            endedAt: null,
            levels: [{ seq: 1, level: "AMARILLO", opened_by: "OPEN", started_at: at("10:00"), ended_at: null }],
        };
        const ticks = buildTimeStrip([activeLeak, motion], NOW)!.ticks;
        expect(ticks.map(tickLabel)).toEqual(["24/08 05:00", "24/08 08:00", "24/08 10:00", "Ahora"]);
    });

    it("la duración de cada barra no cuenta los huecos, como la Duración general", () => {
        const reactivated: StripIndicatorInput = {
            id: 3,
            event: "FUGA",
            endedAt: at("15:00"),
            levels: [
                { seq: 1, level: "AMARILLO", opened_by: "OPEN", started_at: at("05:00"), ended_at: at("07:00") },
                { seq: 2, level: "AMARILLO", opened_by: "REACTIVATION", started_at: at("13:00"), ended_at: at("15:00") },
            ],
        };
        expect(buildTimeStrip([reactivated], NOW)!.bars[0].durationMs).toBe(4 * 3_600_000);
        expect(buildTimeStrip([activeLeak], NOW)!.bars[0].durationMs).toBe(10 * 3_600_000);
    });
});

describe("placeAxisLabels", () => {
    const tick = (offsetPct: number, isNow = false) => ({ atMs: offsetPct, offsetPct, isNow });
    const width = (t: { isNow: boolean }) => (t.isNow ? 40 : 80);

    it("centra la etiqueta sobre su círculo sin salirse de la pista", () => {
        const placed = placeAxisLabels([tick(0), tick(50), tick(100, true)], 400, width, 8);
        expect(placed.map((p) => [p.leftPx, p.centerPx])).toEqual([
            [0, 0],
            [160, 200],
            [360, 400],
        ]);
    });

    it("si chocan, corre la etiqueta a la derecha en vez de ocultarla", () => {
        const placed = placeAxisLabels([tick(0), tick(10), tick(60)], 400, width, 8);
        expect(placed.map((p) => [p.leftPx, p.centerPx])).toEqual([
            [0, 0],
            [88, 40],
            [200, 240],
        ]);
    });

    it("cerca del final retrocede y empuja a las anteriores", () => {
        const placed = placeAxisLabels([tick(0), tick(95), tick(100, true)], 400, width, 8);
        expect(placed.map((p) => p.leftPx)).toEqual([0, 272, 360]);
    });

    it("tres cambios seguidos al inicio caben todos en dos renglones (caso ALR-000052)", () => {
        const placed = placeAxisLabels([tick(0), tick(10), tick(17)], 500, () => 42, 8);
        expect(placed.map((p) => p.leftPx)).toEqual([0, 50, 100]);
    });

    it("si no caben todas, quita las intermedias más apretadas y conserva la primera y Ahora", () => {
        const placed = placeAxisLabels([tick(0), tick(10), tick(20), tick(100, true)], 200, width, 8);
        expect(placed.map((p) => p.tick.offsetPct)).toEqual([0, 100]);
    });

    it("sin ancho medido no coloca nada", () => {
        expect(placeAxisLabels([tick(0)], 0, width, 8)).toEqual([]);
    });
});

describe("textos de la franja", () => {
    const bar = buildTimeStrip([activeLeak], NOW)!.bars[0];

    it("tooltip y etiqueta accesible de un tramo (§10.3)", () => {
        expect(pieceTooltip(bar.pieces[1])).toBe("24/08 08:00 – 24/08 10:00 · Naranja · 2 h");
        expect(pieceAccessibleLabel(bar.pieces[2], "Anomalía acústica")).toBe(
            "Anomalía acústica, Rojo, 24/08 10:00 a ahora, 5 h"
        );
    });

    it("tooltip de los círculos (§5.2)", () => {
        expect(bar.markers.map(markerTooltip)).toEqual([
            "24/08 05:00 · Apertura · Amarillo",
            "24/08 08:00 · Amarillo → Naranja",
            "24/08 10:00 · Naranja → Rojo",
            "Ahora · Rojo",
        ]);
    });

    it("el gris se nombra sin alerta, no como un nivel Normal", () => {
        const motion: StripIndicatorInput = {
            id: 4,
            event: "MOVIMIENTO",
            endedAt: null,
            levels: [{ seq: 1, level: "AMARILLO", opened_by: "OPEN", started_at: at("10:00"), ended_at: null }],
        };
        const idle = buildTimeStrip([activeLeak, motion], NOW)!.bars[1].pieces[0];
        expect(pieceTooltip(idle)).toBe("24/08 05:00 – 24/08 10:00 · Sin alerta · 5 h");
    });
});

describe("cursor de tiempo", () => {
    const strip = () => buildTimeStrip([activeLeak], NOW)!;

    it("una fase marca su rango (CA9)", () => {
        expect(buildStripCursor(strip(), { startMs: ms("08:00"), endMs: ms("10:00") })).toEqual({ offsetPct: 30, widthPct: 20 });
    });

    it("la fase en curso llega hasta ahora", () => {
        const cursor = buildStripCursor(strip(), { startMs: ms("10:00"), endMs: null });
        expect(cursor.offsetPct + cursor.widthPct).toBe(100);
    });

    it("un mensaje es un instante: línea sin ancho", () => {
        expect(buildStripCursor(strip(), { startMs: ms("08:00"), endMs: ms("08:00") })).toEqual({ offsetPct: 30, widthPct: 0 });
    });

    it("un rango fuera del eje se recorta a la franja", () => {
        expect(buildStripCursor(strip(), { startMs: ms("03:00"), endMs: ms("06:00") })).toEqual({ offsetPct: 0, widthPct: 10 });
    });
});

describe("compactStripHeightPx", () => {
    it("≈54 px con una barra y crece por barra", () => {
        expect(compactStripHeightPx(1)).toBe(54);
        expect(compactStripHeightPx(2)).toBe(74);
    });
});
