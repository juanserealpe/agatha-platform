import { describe, expect, it } from "vitest";
import { noticeSummary } from "./noticeSummary";

const BODY = "Índice de ruido en 0,41 desde las 07:00; Rojo empieza en 0,35 y lo normal es menos de 0,20.";

describe("noticeSummary", () => {
    it("una alerta nueva se describe por su nivel, sin cifras", () => {
        expect(noticeSummary({ code: "OPEN", level: "AMARILLO", body: BODY })).toBe("El índice de ruido está por encima de lo normal.");
        expect(noticeSummary({ code: "NEW_AFTER_CLASSIFIED", level: "ROJO", body: BODY })).toBe("El índice de ruido es demasiado alto.");
    });

    it("una subida dice qué cambió", () => {
        expect(noticeSummary({ code: "UP_NARANJA_FAST", level: "NARANJA", body: BODY })).toBe(
            "El índice de ruido está subiendo más rápido de lo normal."
        );
    });

    it("conserva la marca de resultado preliminar", () => {
        const summary = noticeSummary({ code: "OPEN", level: "NARANJA", body: `${BODY} Sensor en ajuste: resultado preliminar.` });
        expect(summary).toBe("El índice de ruido está muy por encima de lo normal. Sensor en ajuste: resultado preliminar.");
    });
});
