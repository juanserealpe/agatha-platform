import { describe, expect, it } from "vitest";
import { buildClassificationRequest, buildLabelRequest, buildObservationRequest } from "./managementRequests";

// JSON.stringify elimina los campos undefined: así se compara lo que realmente viaja al backend.
const sent = (body: object) => JSON.parse(JSON.stringify(body));

describe("buildClassificationRequest", () => {
    it("envía origen WEB y la observación recortada", () => {
        expect(sent(buildClassificationRequest({ classification: "CONFIRMADA", observation: "  humedad  " }, "uuid-1"))).toEqual({
            classification: "CONFIRMADA",
            observation: "humedad",
            origin: "WEB",
            client_record_id: "uuid-1",
        });
    });

    it("omite la observación vacía y el UUID ausente", () => {
        expect(sent(buildClassificationRequest({ classification: "FALSA_ALERTA", observation: "   " }))).toEqual({
            classification: "FALSA_ALERTA",
            origin: "WEB",
        });
    });
});

describe("buildLabelRequest", () => {
    it("con OTRO envía el texto libre", () => {
        expect(sent(buildLabelRequest({ label: "OTRO", otherText: " raíz de árbol ", observation: "" }))).toEqual({
            label: "OTRO",
            other_text: "raíz de árbol",
            origin: "WEB",
        });
    });

    it("con otra etiqueta NO envía other_text aunque haya quedado escrito", () => {
        expect(sent(buildLabelRequest({ label: "CLIMA_EXTREMO", otherText: "texto viejo", observation: "lluvia" }))).toEqual({
            label: "CLIMA_EXTREMO",
            observation: "lluvia",
            origin: "WEB",
        });
    });
});

describe("buildObservationRequest", () => {
    it("recorta el texto y envía origen WEB", () => {
        expect(sent(buildObservationRequest({ text: " visita mañana " }, "uuid-2"))).toEqual({
            text: "visita mañana",
            origin: "WEB",
            client_record_id: "uuid-2",
        });
    });
});
