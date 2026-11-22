import { describe, expect, it, vi } from "vitest";
import { toAlertId, type AlertLevel, type DayRange, type LevelSegment } from "@/services/alertManagement";
import {
    alertSpanMs,
    connectedGroup,
    findAlertGroup,
    MAX_GROUP_ROUNDS,
    memberCurrentLevel,
    spanToDays,
    summarizeGroup,
    type GroupCandidate,
} from "./alertGroup";

const HOUR_MS = 3_600_000;
const NOW = new Date("2026-11-21T15:00:00-05:00").getTime();

type Piece = [level: AlertLevel, from: string, to: string | null];

function segments(pieces: readonly Piece[]): LevelSegment[] {
    return pieces.map(([level, started_at, ended_at], index) => ({
        seq: index + 1,
        opened_by: index === 0 ? "OPEN" : "UP",
        level,
        started_at,
        ended_at,
        messages: [],
    }));
}

/** Alerta de prueba. Sin `to` está activa y su último tramo sigue abierto. */
function alert(id: number, from: string, to: string | null, overrides: Partial<GroupCandidate> = {}): GroupCandidate {
    const levels = overrides.levels ?? segments([["AMARILLO", from, to]]);
    return {
        id: toAlertId(id),
        code: `ALR-${String(id).padStart(6, "0")}`,
        event: "FUGA",
        state: to === null ? "ACTIVA" : "FINALIZADA",
        started_at: from,
        ended_at: to,
        max_level: "AMARILLO",
        classification: null,
        ...overrides,
        levels,
    };
}

const ids = (members: readonly GroupCandidate[]) => members.map((member) => member.id);

describe("alertSpanMs", () => {
    it("una alerta activa llega hasta ahora", () => {
        const span = alertSpanMs(alert(1, "2026-11-21T10:00:00-05:00", null), NOW);
        expect(span.endMs).toBe(NOW);
    });
});

describe("connectedGroup", () => {
    // Ejemplo de la spec: fuga 18→21-nov y movimiento 19→20-nov en el mismo punto.
    const leak = alert(120, "2026-11-18T08:00:00-05:00", "2026-11-21T10:00:00-05:00");
    const motion = alert(125, "2026-11-19T14:00:00-05:00", "2026-11-20T16:00:00-05:00", { event: "MOVIMIENTO" });

    it("encuentra la que empezó antes y sigue cruzándose", () => {
        expect(ids(connectedGroup(motion, [leak, motion], NOW))).toEqual([120, 125]);
    });

    it("da el mismo grupo desde cualquiera de sus miembros", () => {
        expect(ids(connectedGroup(leak, [leak, motion], NOW))).toEqual(ids(connectedGroup(motion, [motion, leak], NOW)));
    });

    it("encadena: entra la que se cruza con cualquier miembro, aunque no con la abierta", () => {
        const longMotion = alert(30, "2026-11-10T00:00:00-05:00", "2026-11-15T00:00:00-05:00", { event: "MOVIMIENTO" });
        const firstLeak = alert(31, "2026-11-09T00:00:00-05:00", "2026-11-11T00:00:00-05:00");
        const secondLeak = alert(33, "2026-11-14T00:00:00-05:00", null);
        expect(ids(connectedGroup(firstLeak, [secondLeak, longMotion, firstLeak], NOW))).toEqual([31, 30, 33]);
    });

    it("no agrupa con tolerancia: una alerta que empieza una hora después del fin queda fuera", () => {
        const later = alert(126, "2026-11-21T11:00:00-05:00", null);
        expect(ids(connectedGroup(leak, [later], NOW))).toEqual([120]);
    });

    it("tocarse en el borde cuenta como cruce", () => {
        const next = alert(127, "2026-11-21T10:00:00-05:00", null);
        expect(ids(connectedGroup(leak, [next], NOW))).toEqual([120, 127]);
    });

    it("la versión de la alerta abierta prevalece sobre la de la lista", () => {
        const stale = { ...motion, max_level: "ROJO" as const };
        const [, opened] = connectedGroup(motion, [leak, stale], NOW);
        expect(opened).toBe(motion);
    });

    it("siempre incluye la abierta aunque la lista no la traiga", () => {
        expect(ids(connectedGroup(motion, [], NOW))).toEqual([125]);
    });
});

describe("spanToDays", () => {
    it("usa los días de Colombia aunque en UTC ya sea el día siguiente", () => {
        const span = { startMs: new Date("2026-11-19T22:30:00-05:00").getTime(), endMs: new Date("2026-11-20T23:30:00-05:00").getTime() };
        expect(spanToDays(span)).toEqual({ from: "2026-11-19", to: "2026-11-20" });
    });
});

describe("findAlertGroup", () => {
    it("con una alerta sola hace una consulta con sus días y 3 de margen (sin pasar de hoy)", async () => {
        const only = alert(1, "2026-11-20T08:00:00-05:00", "2026-11-20T20:00:00-05:00");
        const fetchDays = vi.fn(async () => [only]);
        await expect(findAlertGroup(only, fetchDays, NOW)).resolves.toEqual({ members: [only], complete: true });
        expect(fetchDays).toHaveBeenCalledTimes(1);
        expect(fetchDays).toHaveBeenCalledWith({ from: "2026-11-17", to: "2026-11-21" });
    });

    // Cadena: ALR-125 se cruza con ALR-120, y ALR-120 con ALR-110 (que no toca a ALR-125).
    const chain = () => {
        const leak = alert(120, "2026-11-18T08:00:00-05:00", "2026-11-21T10:00:00-05:00");
        const motion = alert(125, "2026-11-19T14:00:00-05:00", "2026-11-20T16:00:00-05:00");
        const earlier = alert(110, "2026-11-17T08:00:00-05:00", "2026-11-18T09:00:00-05:00");
        const all = [leak, motion, earlier];
        // Como el backend: devuelve las que se cruzan con los días pedidos (no solo las que empezaron en ellos).
        const fetchDays = vi.fn(async ({ from, to }: DayRange) =>
            all.filter((a) => spanToDays(alertSpanMs(a, NOW)).from <= to && spanToDays(alertSpanMs(a, NOW)).to >= from)
        );
        return { motion, fetchDays };
    };

    it("con el margen, una cadena normal llega en una sola consulta", async () => {
        const { motion, fetchDays } = chain();
        const result = await findAlertGroup(motion, fetchDays, NOW);
        expect(ids(result.members)).toEqual([110, 120, 125]);
        expect(fetchDays).toHaveBeenCalledTimes(1);
    });

    it("amplía el rango cuando un miembro empezó antes y vuelve a consultar", async () => {
        const { motion, fetchDays } = chain();
        const result = await findAlertGroup(motion, fetchDays, NOW, MAX_GROUP_ROUNDS, 0);
        expect(ids(result.members)).toEqual([110, 120, 125]);
        expect(result.complete).toBe(true);
        expect(fetchDays.mock.calls.map(([days]) => days)).toEqual([
            { from: "2026-11-19", to: "2026-11-20" },
            { from: "2026-11-18", to: "2026-11-21" },
            { from: "2026-11-17", to: "2026-11-21" },
        ]);
    });

    it("descarta las del mismo día que no se cruzan por hora", async () => {
        const morning = alert(1, "2026-11-20T01:00:00-05:00", "2026-11-20T03:00:00-05:00");
        const night = alert(2, "2026-11-20T20:00:00-05:00", "2026-11-20T22:00:00-05:00");
        const result = await findAlertGroup(night, async () => [morning, night], NOW);
        expect(ids(result.members)).toEqual([2]);
    });

    it("se detiene en el tope de vueltas y avisa que puede faltar", async () => {
        // Cadena que se estira un día por consulta: cada respuesta trae una alerta que empezó el día anterior.
        const seed = alert(100, "2026-11-20T08:00:00-05:00", "2026-11-20T20:00:00-05:00");
        let round = 0;
        const fetchDays = vi.fn(async () => {
            round++;
            const start = new Date(new Date(seed.started_at).getTime() - round * 24 * HOUR_MS).toISOString();
            return [seed, alert(100 + round, start, seed.started_at)];
        });
        const result = await findAlertGroup(seed, fetchDays, NOW, 3, 0);
        expect(fetchDays).toHaveBeenCalledTimes(3);
        expect(result.complete).toBe(false);
    });

    it("propaga el error de la consulta", async () => {
        const seed = alert(1, "2026-11-20T08:00:00-05:00", null);
        await expect(findAlertGroup(seed, async () => Promise.reject(new Error("caído")), NOW)).rejects.toThrow("caído");
    });
});

describe("memberCurrentLevel", () => {
    it("es el tramo abierto de una activa y null en una finalizada", () => {
        const active = alert(1, "2026-11-21T08:00:00-05:00", null, {
            levels: segments([
                ["AMARILLO", "2026-11-21T08:00:00-05:00", "2026-11-21T10:00:00-05:00"],
                ["NARANJA", "2026-11-21T10:00:00-05:00", null],
            ]),
        });
        expect(memberCurrentLevel(active)).toBe("NARANJA");
        expect(memberCurrentLevel(alert(2, "2026-11-20T08:00:00-05:00", "2026-11-20T09:00:00-05:00"))).toBeNull();
    });
});

describe("summarizeGroup", () => {
    const redThenYellow = alert(1, "2026-11-21T05:00:00-05:00", null, {
        max_level: "ROJO",
        levels: segments([
            ["ROJO", "2026-11-21T05:00:00-05:00", "2026-11-21T09:00:00-05:00"],
            ["AMARILLO", "2026-11-21T09:00:00-05:00", null],
        ]),
    });
    const motion = alert(2, "2026-11-21T07:00:00-05:00", "2026-11-21T12:00:00-05:00", { event: "MOVIMIENTO" });

    it("en curso: nivel actual más alto entre las activas, no el máximo alcanzado", () => {
        const summary = summarizeGroup([redThenYellow, motion], NOW);
        expect(summary).toMatchObject({ size: 2, inProgress: true, level: "AMARILLO", blinking: false });
    });

    it("finalizado: el máximo alcanzado del grupo", () => {
        const finished = { ...redThenYellow, state: "FINALIZADA" as const, ended_at: "2026-11-21T13:00:00-05:00" };
        expect(summarizeGroup([finished, motion], NOW)).toMatchObject({ inProgress: false, level: "ROJO" });
    });

    it("titila si alguna activa está en Rojo sin clasificar", () => {
        const red = alert(3, "2026-11-21T10:00:00-05:00", null, {
            max_level: "ROJO",
            levels: segments([["ROJO", "2026-11-21T10:00:00-05:00", null]]),
        });
        expect(summarizeGroup([red, motion], NOW).blinking).toBe(true);
        expect(summarizeGroup([{ ...red, classification: "CONFIRMADA" }], NOW).blinking).toBe(false);
    });

    it("duración general: lo cubierto por el grupo, sin contar dos veces lo que se cruza", () => {
        // Fuga 05:00 → ahora (15:00) y movimiento 07:00 → 12:00: 10 h, no 15 h.
        expect(summarizeGroup([redThenYellow, motion], NOW).durationMs).toBe(10 * HOUR_MS);
    });

    it("con una alerta sola no cuenta el hueco entre reactivaciones, como la línea de tiempo", () => {
        const reactivated = alert(4, "2026-11-20T08:00:00-05:00", "2026-11-20T20:00:00-05:00", {
            levels: segments([
                ["AMARILLO", "2026-11-20T08:00:00-05:00", "2026-11-20T10:00:00-05:00"],
                ["AMARILLO", "2026-11-20T18:00:00-05:00", "2026-11-20T20:00:00-05:00"],
            ]),
        });
        expect(summarizeGroup([reactivated], NOW).durationMs).toBe(4 * HOUR_MS);
    });
});
