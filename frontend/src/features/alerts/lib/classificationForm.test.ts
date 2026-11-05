import { describe, expect, it } from "vitest";
import { allowedClassifications, validateClassificationForm } from "./classificationForm";

const BOTH = ["CONFIRMADA", "FALSA_ALERTA"] as const;

describe("allowedClassifications", () => {
    it("ofrece las dos opciones cuando el backend las habilita", () => {
        expect(allowedClassifications(["CONFIRM", "FALSE_ALERT", "VIEW"])).toEqual(["CONFIRMADA", "FALSA_ALERTA"]);
    });

    it("solo la habilitada si llega una", () => {
        expect(allowedClassifications(["FALSE_ALERT", "VIEW"])).toEqual(["FALSA_ALERTA"]);
    });

    it("ninguna en una alerta ya clasificada", () => {
        expect(allowedClassifications(["ADD_LABEL", "VIEW"])).toEqual([]);
    });
});

describe("validateClassificationForm", () => {
    it("sin opción preseleccionada no se puede guardar", () => {
        expect(validateClassificationForm({ classification: null, observation: "" }, BOTH)).toEqual(["classification"]);
    });

    it("una opción habilitada es válida y la observación es opcional", () => {
        expect(validateClassificationForm({ classification: "FALSA_ALERTA", observation: "" }, BOTH)).toEqual([]);
    });

    it("rechaza una opción que el backend no habilitó", () => {
        expect(validateClassificationForm({ classification: "CONFIRMADA", observation: "" }, ["FALSA_ALERTA"])).toEqual([
            "classification",
        ]);
    });

    it("la observación admite hasta 500 caracteres", () => {
        expect(validateClassificationForm({ classification: "CONFIRMADA", observation: "x".repeat(501) }, BOTH)).toEqual([
            "observation",
        ]);
    });
});
