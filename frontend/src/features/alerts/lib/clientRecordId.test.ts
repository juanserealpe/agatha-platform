import { describe, expect, it } from "vitest";
import { createClientRecordIdTracker } from "./clientRecordId";

function sequence() {
    let n = 0;
    return () => `uuid-${++n}`;
}

describe("createClientRecordIdTracker", () => {
    it("reutiliza el UUID al reintentar el mismo guardado", () => {
        const tracker = createClientRecordIdTracker(sequence());
        const payload = { classification: "CONFIRMADA", observation: "x" };

        expect(tracker.idFor(payload)).toBe("uuid-1");
        expect(tracker.idFor({ ...payload })).toBe("uuid-1");
    });

    it("estrena UUID si cambia lo que se envía", () => {
        const tracker = createClientRecordIdTracker(sequence());

        expect(tracker.idFor({ observation: "a" })).toBe("uuid-1");
        expect(tracker.idFor({ observation: "b" })).toBe("uuid-2");
    });

    it("estrena UUID después de un guardado exitoso", () => {
        const tracker = createClientRecordIdTracker(sequence());
        const payload = { text: "x" };

        tracker.idFor(payload);
        tracker.reset();
        expect(tracker.idFor(payload)).toBe("uuid-2");
    });

    it("sin generador disponible omite el UUID", () => {
        const tracker = createClientRecordIdTracker(() => undefined);
        expect(tracker.idFor({})).toBeUndefined();
    });
});
