import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/services/http";
import { httpAlertManagementService as service } from "./alertManagement.http";
import { parseAlertId, toAlertId } from "./alertId";
import type { AlertSearchFilters } from "./alertManagement.types";

const NO_FILTERS: AlertSearchFilters = { device: null, from: null, to: null, maxLevels: [], events: [], statuses: [] };
const ALERT_ID = toAlertId(123);

function jsonResponse(status: number, body: unknown): Response {
    return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

function mockFetch(response: Response) {
    const fetchMock = vi.fn().mockResolvedValue(response);
    vi.stubGlobal("fetch", fetchMock);
    return fetchMock;
}

function requestedUrl(fetchMock: ReturnType<typeof vi.fn>): string {
    return fetchMock.mock.calls[0][0] as string;
}

afterEach(() => {
    vi.unstubAllGlobals();
});

describe("search", () => {
    it("envía solo los filtros con valor y la página", async () => {
        const fetchMock = mockFetch(jsonResponse(200, { total: 0, page: 2, page_size: 10, items: [] }));

        await service.search(
            { ...NO_FILTERS, device: 11556, from: "2026-08-01", maxLevels: ["NARANJA", "ROJO"], statuses: ["SIN_CLASIFICAR"] },
            2
        );

        expect(requestedUrl(fetchMock)).toBe(
            "/api/alerts?device=11556&from=2026-08-01&max_level=NARANJA&max_level=ROJO&status=SIN_CLASIFICAR&page=2"
        );
    });
});

describe("getPending", () => {
    it("omite limit si no se pide (el backend usa 5)", async () => {
        const fetchMock = mockFetch(jsonResponse(200, { total: 0, items: [] }));

        await service.getPending();

        expect(requestedUrl(fetchMock)).toBe("/api/alerts/pending");
    });

    it("envía limit y devuelve el total junto con las filas", async () => {
        const body = { total: 12, items: [{ id: 123 }] };
        const fetchMock = mockFetch(jsonResponse(200, body));

        await expect(service.getPending(20)).resolves.toEqual(body);
        expect(requestedUrl(fetchMock)).toBe("/api/alerts/pending?limit=20");
    });
});

describe("getActive", () => {
    it("consulta /active y devuelve los puntos tal cual", async () => {
        const body = {
            devices: [{ measuring_point_id: 11556, pk: "Sensor PK34-470", level: "ROJO", blinking: true, alerts: [] }],
        };
        const fetchMock = mockFetch(jsonResponse(200, body));

        await expect(service.getActive()).resolves.toEqual(body);
        expect(requestedUrl(fetchMock)).toBe("/api/alerts/active");
    });
});

describe("getDeviceAlerts", () => {
    it("consulta las alertas del DISPOSITIVO con el rango de días", async () => {
        const body = { measuring_point_id: 11556, pk: "Sensor PK34-470", alerts: [] };
        const fetchMock = mockFetch(jsonResponse(200, body));

        await expect(service.getDeviceAlerts(11556, { from: "2026-11-18", to: "2026-11-21" })).resolves.toEqual(body);
        expect(requestedUrl(fetchMock)).toBe("/api/alerts/devices/11556/alerts?from=2026-11-18&to=2026-11-21");
    });
});

describe("exportXlsx", () => {
    it("usa los mismos filtros, sin página", async () => {
        const fetchMock = mockFetch(new Response("xlsx", { status: 200 }));

        await service.exportXlsx({ ...NO_FILTERS, events: ["FUGA"] });

        expect(requestedUrl(fetchMock)).toBe("/api/alerts/export.xlsx?event=FUGA");
    });

    it("rechaza con el mensaje del backend si la búsqueda es muy grande", async () => {
        const message = "La búsqueda tiene más de 10.000 alertas. Ajuste los filtros para exportar.";
        mockFetch(jsonResponse(400, { code: "INVALID_INPUT", message }));

        await expect(service.exportXlsx(NO_FILTERS)).rejects.toMatchObject({ status: 400, message });
    });
});

describe("escrituras de gestión", () => {
    it("usa el ID de la alerta en la ruta y envía exactamente el cuerpo recibido", async () => {
        const fetchMock = mockFetch(jsonResponse(201, { outcome: "SAVED", alert: { id: 123 } }));

        await service.classify(ALERT_ID, { classification: "CONFIRMADA", origin: "WEB" });

        expect(requestedUrl(fetchMock)).toBe("/api/alerts/123/classification");
        expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual({ classification: "CONFIRMADA", origin: "WEB" });
    });

    it("trata el 409 con outcome como resultado (otro usuario gestionó primero)", async () => {
        const body = {
            code: "CONFLICT",
            outcome: "SAVED_AS_CONFLICT_OBSERVATION",
            alert: { id: 123 },
            existing: { classification: "CONFIRMADA", origin: "WEB" },
        };
        mockFetch(jsonResponse(409, body));

        await expect(service.label(ALERT_ID, { label: "CLIMA_EXTREMO", origin: "WEB" })).resolves.toEqual(body);
    });

    it("rechaza el 409 sin outcome conservando el message", async () => {
        const message = "la alerta todavía no está clasificada: primero clasificar, después etiquetar";
        mockFetch(jsonResponse(409, { code: "CONFLICT", message }));

        const error = await service.label(ALERT_ID, { label: "OTRO", other_text: "x", origin: "WEB" }).catch((e: unknown) => e);
        expect(error).toBeInstanceOf(ApiError);
        expect(error).toMatchObject({ status: 409, code: "CONFLICT", message });
    });
});

describe("parseAlertId", () => {
    it.each([
        ["123", 123],
        [" 7 ", 7],
    ])("%s → %d", (raw, expected) => {
        expect(parseAlertId(raw)).toBe(expected);
    });

    it.each([null, "", "0", "-1", "1.5", "ALR-000123", "abc", "99999999999999999999"])("%s → null", (raw) => {
        expect(parseAlertId(raw)).toBeNull();
    });
});
