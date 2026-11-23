import { describe, expect, it } from "vitest";
import { toAlertId, type AlertLevel } from "@/services/alertManagement";
import type { GroupCandidate } from "./alertGroup";
import { splitTabs, tabSignature, tabsWithNovelty } from "./alertTabs";

/** Alerta de prueba que empieza el `day` de noviembre. Sin `ended` está activa en `level`. */
function alert(id: number, day: number, ended: boolean, level: AlertLevel = "AMARILLO"): GroupCandidate {
    const started_at = `2026-11-${String(day).padStart(2, "0")}T08:00:00-05:00`;
    const ended_at = ended ? `2026-11-${String(day).padStart(2, "0")}T20:00:00-05:00` : null;
    return {
        id: toAlertId(id),
        code: `ALR-${String(id).padStart(6, "0")}`,
        event: "FUGA",
        state: ended ? "FINALIZADA" : "ACTIVA",
        started_at,
        ended_at,
        max_level: level,
        classification: null,
        levels: [{ seq: 1, opened_by: "OPEN", level, started_at, ended_at, messages: [] }],
    };
}

const ids = (members: readonly GroupCandidate[]) => members.map((member) => member.id);

describe("splitTabs", () => {
    it("con 3 alertas o menos se ven todas y no hay menú", () => {
        const members = [alert(1, 10, true), alert(2, 11, true), alert(3, 12, false)];
        expect(splitTabs(members, 1)).toEqual({ visible: members, overflow: [] });
    });

    it("prioriza la abierta y las activas; el resto va al menú, en orden cronológico", () => {
        const members = [alert(1, 10, true), alert(2, 11, false), alert(3, 12, true), alert(4, 13, true), alert(5, 14, false)];
        const split = splitTabs(members, 1);
        expect(ids(split.visible)).toEqual([1, 2, 5]);
        expect(ids(split.overflow)).toEqual([3, 4]);
    });

    it("si sobra lugar lo llenan las finalizadas más recientes", () => {
        const members = [alert(1, 10, true), alert(2, 11, true), alert(3, 12, true), alert(4, 13, true)];
        expect(ids(splitTabs(members, 2).visible)).toEqual([2, 3, 4]);
    });

    it("con más activas que lugares quedan las más recientes", () => {
        const members = [alert(1, 10, false), alert(2, 11, false), alert(3, 12, false), alert(4, 13, false), alert(5, 14, true)];
        const split = splitTabs(members, 5);
        expect(ids(split.visible)).toEqual([3, 4, 5]);
        expect(ids(split.overflow)).toEqual([1, 2]);
    });
});

describe("novedad de las pestañas", () => {
    it("la firma cambia con el nivel actual y con el estado", () => {
        const active = alert(1, 10, false, "NARANJA");
        expect(tabSignature(active)).toBe("ACTIVA|NARANJA");
        expect(tabSignature(alert(1, 10, true, "ROJO"))).toBe("FINALIZADA|ROJO");
    });

    it("marca las que cambiaron o aparecieron, nunca la abierta", () => {
        const before = [alert(1, 10, false), alert(2, 11, false)];
        const seen = new Map(before.map((member) => [member.id as number, tabSignature(member)]));
        const after = [alert(1, 10, false, "ROJO"), alert(2, 11, false, "NARANJA"), alert(3, 12, false)];
        expect([...tabsWithNovelty(after, seen, 1)]).toEqual([2, 3]);
        expect(tabsWithNovelty(before, seen, 1).size).toBe(0);
    });
});
