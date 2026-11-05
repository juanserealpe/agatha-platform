import { describe, expect, it } from "vitest";
import { LABELS_BY_CLASSIFICATION } from "@/services/alertManagement";
import { SEVERITY_STYLES } from "./alertDisplay";
import { displayLevel, LABEL_TEXT, labelDisplayText, levelStyle } from "./alertManagementDisplay";

describe("levelStyle", () => {
    it("reutiliza la escala de severidad de la app", () => {
        expect(levelStyle("ROJO")).toBe(SEVERITY_STYLES.ROJO);
        expect(levelStyle("AMARILLO")).toBe(SEVERITY_STYLES.AMARILLO);
    });
});

describe("displayLevel", () => {
    it("usa el nivel actual y, si la alerta finalizó, el máximo", () => {
        expect(displayLevel({ current_level: "NARANJA", max_level: "ROJO" })).toBe("NARANJA");
        expect(displayLevel({ current_level: null, max_level: "ROJO" })).toBe("ROJO");
    });
});

describe("etiquetas", () => {
    it("cada clasificación ofrece sus tres etiquetas más Otro", () => {
        expect(LABELS_BY_CLASSIFICATION.CONFIRMADA).toHaveLength(4);
        expect(LABELS_BY_CLASSIFICATION.FALSA_ALERTA).toHaveLength(4);
        expect(LABELS_BY_CLASSIFICATION.CONFIRMADA).toContain("OTRO");
        expect(LABELS_BY_CLASSIFICATION.FALSA_ALERTA).toContain("OTRO");
    });

    it("las etiquetas de una clasificación no aparecen en la otra, salvo Otro", () => {
        const confirmed = new Set<string>(LABELS_BY_CLASSIFICATION.CONFIRMADA);
        const shared = LABELS_BY_CLASSIFICATION.FALSA_ALERTA.filter((label) => confirmed.has(label));
        expect(shared).toEqual(["OTRO"]);
    });

    it("todas las etiquetas permitidas tienen texto", () => {
        [...LABELS_BY_CLASSIFICATION.CONFIRMADA, ...LABELS_BY_CLASSIFICATION.FALSA_ALERTA].forEach((label) => {
            expect(LABEL_TEXT[label]).toBeTruthy();
        });
    });
});

describe("labelDisplayText", () => {
    it("null si la alerta no tiene etiqueta", () => {
        expect(labelDisplayText(null, null)).toBeNull();
    });

    it("usa el texto del catálogo", () => {
        expect(labelDisplayText("TRANSITO_PESADO", null)).toBe("Tránsito de vehículo pesado");
    });

    it("con Otro muestra el texto libre", () => {
        expect(labelDisplayText("OTRO", "  prueba de presión ")).toBe("Otro: prueba de presión");
    });

    it("con Otro sin texto no deja un hueco", () => {
        expect(labelDisplayText("OTRO", null)).toBe("Otro");
        expect(labelDisplayText("OTRO", "   ")).toBe("Otro");
    });
});
