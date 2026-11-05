import { describe, expect, it } from "vitest";
import type { AlertAction } from "@/services/alertManagement";
import { getRowActions } from "./rowActions";

describe("getRowActions", () => {
    it("agrupa Confirmar y Falsa alerta en un único Clasificar", () => {
        expect(getRowActions(["CONFIRM", "FALSE_ALERT", "VIEW"])).toEqual(["classify", "view"]);
    });

    it("muestra Clasificar aunque llegue solo una de las dos acciones", () => {
        expect(getRowActions(["FALSE_ALERT"])).toEqual(["classify"]);
    });

    it("clasificada sin etiqueta: Agregar etiqueta y Ver detalle", () => {
        expect(getRowActions(["ADD_LABEL", "VIEW"])).toEqual(["label", "view"]);
    });

    it("clasificada con etiqueta: solo Ver detalle", () => {
        expect(getRowActions(["VIEW"])).toEqual(["view"]);
    });

    it("ordena la acción pendiente primero sin importar el orden del backend", () => {
        expect(getRowActions(["VIEW", "ADD_LABEL"])).toEqual(["label", "view"]);
    });

    it("ignora acciones desconocidas y listas vacías", () => {
        expect(getRowActions(["ARCHIVE" as AlertAction, "VIEW"])).toEqual(["view"]);
        expect(getRowActions([])).toEqual([]);
    });
});

