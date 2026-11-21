import { describe, expect, it } from "vitest";
import { messageVariant } from "./timelineMessage";

describe("messageVariant", () => {
    it("marca los TEMP_* como evento del sensor, sea Notificación o Actualización", () => {
        expect(messageVariant({ kind: "NOTIFICACION", code: "TEMP_STABILIZED" })).toBe("sensor");
        expect(messageVariant({ kind: "NOTIFICACION", code: "TEMP_VARIES_AGAIN" })).toBe("sensor");
        expect(messageVariant({ kind: "ACTUALIZACION", code: "TEMP_NOT_STABILIZED" })).toBe("sensor");
        expect(messageVariant({ kind: "ACTUALIZACION", code: "TEMP_STILL_STABLE" })).toBe("sensor");
    });

    it("marca la clasificación y la etiqueta como gestión", () => {
        expect(messageVariant({ kind: "GESTION", code: "CLASSIFIED" })).toBe("management");
        expect(messageVariant({ kind: "GESTION", code: "LABELED" })).toBe("management");
    });

    it("deja los mensajes del indicador en la variante normal", () => {
        expect(messageVariant({ kind: "NOTIFICACION", code: "OPEN" })).toBe("default");
        expect(messageVariant({ kind: "ACTUALIZACION", code: "MOV_DISPLACED" })).toBe("default");
        expect(messageVariant({ kind: "ACTUALIZACION", code: "MOV_RETURN" })).toBe("default");
        expect(messageVariant({ kind: "FIN", code: "END_NORMAL" })).toBe("default");
    });
});
