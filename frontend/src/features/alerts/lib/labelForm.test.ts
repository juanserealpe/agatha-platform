import { describe, expect, it } from "vitest";
import { LABELS_BY_CLASSIFICATION } from "@/services/alertManagement";
import { validateLabelForm, type LabelFormValues } from "./labelForm";
import { describeExisting } from "./managementOutcome";
import { parseManagementMode, ROW_ACTION_MODE } from "./managementMode";

const CONFIRMED = LABELS_BY_CLASSIFICATION.CONFIRMADA;
const EMPTY: LabelFormValues = { label: null, otherText: "", observation: "" };

describe("validateLabelForm", () => {
    it("exige elegir una etiqueta", () => {
        expect(validateLabelForm(EMPTY, CONFIRMED)).toEqual(["label"]);
    });

    it("una etiqueta de su clasificación es válida", () => {
        expect(validateLabelForm({ ...EMPTY, label: "MOVIMIENTO_TIERRA" }, CONFIRMED)).toEqual([]);
    });

    it("rechaza una etiqueta de la otra clasificación", () => {
        expect(validateLabelForm({ ...EMPTY, label: "CLIMA_EXTREMO" }, CONFIRMED)).toEqual(["label"]);
    });

    it("Otro exige texto de 1 a 100 caracteres", () => {
        expect(validateLabelForm({ ...EMPTY, label: "OTRO", otherText: "   " }, CONFIRMED)).toEqual(["otherText"]);
        expect(validateLabelForm({ ...EMPTY, label: "OTRO", otherText: "x".repeat(101) }, CONFIRMED)).toEqual(["otherText"]);
        expect(validateLabelForm({ ...EMPTY, label: "OTRO", otherText: "x".repeat(100) }, CONFIRMED)).toEqual([]);
    });

    it("el texto de Otro no importa con otra etiqueta", () => {
        expect(validateLabelForm({ ...EMPTY, label: "MAQUINARIA_AJENA", otherText: "" }, CONFIRMED)).toEqual([]);
    });

    it("la observación admite hasta 500 caracteres", () => {
        expect(validateLabelForm({ ...EMPTY, label: "MAQUINARIA_AJENA", observation: "x".repeat(500) }, CONFIRMED)).toEqual([]);
        expect(validateLabelForm({ ...EMPTY, label: "MAQUINARIA_AJENA", observation: "x".repeat(501) }, CONFIRMED)).toEqual([
            "observation",
        ]);
    });
});

describe("describeExisting", () => {
    it("clasificación previa", () => {
        expect(describeExisting({ classification: "CONFIRMADA", origin: "WEB" })).toBe("Ya estaba clasificada como Confirmada desde Web.");
    });

    it("etiqueta previa, incluido Otro", () => {
        expect(describeExisting({ label: "TRANSITO_PESADO", origin: "APP_MOVIL" })).toBe(
            "Ya estaba etiquetada como «Tránsito de vehículo pesado» desde App móvil."
        );
        expect(describeExisting({ label: "OTRO", label_other_text: "raíz", origin: "WEB" })).toBe(
            "Ya estaba etiquetada como «Otro: raíz» desde Web."
        );
    });

    it("sin datos de lo existente da un aviso genérico", () => {
        expect(describeExisting(undefined)).toMatch(/Otro usuario/);
    });
});

describe("modos del modal", () => {
    it("cada acción de fila abre su modo", () => {
        expect(ROW_ACTION_MODE).toEqual({ classify: "clasificar", label: "etiquetar", view: "lectura" });
    });

    it("un modo desconocido abre en lectura", () => {
        expect(parseManagementMode("etiquetar")).toBe("etiquetar");
        expect(parseManagementMode("borrar")).toBe("lectura");
        expect(parseManagementMode(null)).toBe("lectura");
    });
});
