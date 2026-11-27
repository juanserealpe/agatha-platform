import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { memoryStorage } from "@/services/auth/testing";
import { tokenStore } from "@/services/auth/tokenStore";
import { ApiError, NETWORK_ERROR_CODE } from "./apiError";
import { apiPath, buildQueryString, httpClient, SESSION_EXPIRED_EVENT } from "./httpClient";

function jsonResponse(status: number, body: unknown): Response {
    return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

afterEach(() => {
    vi.unstubAllGlobals();
});

describe("buildQueryString", () => {
    it("omite valores vacíos y repite los arreglos", () => {
        expect(
            buildQueryString({ device: 11556, from: "", to: null, page: undefined, max_level: ["NARANJA", "ROJO"], event: [] })
        ).toBe("?device=11556&max_level=NARANJA&max_level=ROJO");
    });

    it("codifica los valores", () => {
        expect(buildQueryString({ q: "a&b=c" })).toBe("?q=a%26b%3Dc");
    });

    it("devuelve cadena vacía si no hay parámetros", () => {
        expect(buildQueryString({})).toBe("");
    });
});

describe("apiPath", () => {
    it("codifica cada segmento", () => {
        expect(apiPath("api", "alerts", "ALR/1?x", "label")).toBe("/api/alerts/ALR%2F1%3Fx/label");
    });
});

describe("httpClient", () => {
    it("envía JSON con las cabeceras del contrato", async () => {
        const fetchMock = vi.fn().mockResolvedValue(jsonResponse(201, { outcome: "SAVED" }));
        vi.stubGlobal("fetch", fetchMock);

        const result = await httpClient.post("/api/x", { origin: "WEB" });

        expect(result).toEqual({ status: 201, data: { outcome: "SAVED" } });
        const [, init] = fetchMock.mock.calls[0];
        expect(init.method).toBe("POST");
        expect(init.body).toBe('{"origin":"WEB"}');
        expect(init.headers).toMatchObject({ "Content-Type": "application/json" });
    });

    it("usa el message del backend en el error", async () => {
        vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse(400, { code: "INVALID_INPUT", message: "Fecha inválida" })));

        await expect(httpClient.get("/api/x")).rejects.toMatchObject({ status: 400, code: "INVALID_INPUT", message: "Fecha inválida" });
    });

    it("usa el mensaje de respaldo si el backend no envía uno", async () => {
        vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response("Bad gateway", { status: 502 })));

        await expect(httpClient.get("/api/x", { errorMessage: "No se pudo" })).rejects.toMatchObject({
            status: 502,
            code: "HTTP_502",
            message: "No se pudo",
        });
    });

    it("convierte un fallo de red en un error mostrable", async () => {
        vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new TypeError("Failed to fetch")));

        const error = await httpClient.get("/api/x").catch((e: unknown) => e);
        expect(error).toBeInstanceOf(ApiError);
        expect(error).toMatchObject({ status: 0, code: NETWORK_ERROR_CODE });
    });

    it("propaga la cancelación sin convertirla en error de red", async () => {
        const controller = new AbortController();
        controller.abort();
        const abortError = new DOMException("Aborted", "AbortError");
        vi.stubGlobal("fetch", vi.fn().mockRejectedValue(abortError));

        await expect(httpClient.get("/api/x", { signal: controller.signal })).rejects.toBe(abortError);
    });

    it("devuelve el blob y el nombre del archivo", async () => {
        const response = new Response("xlsx", {
            status: 200,
            headers: { "Content-Disposition": 'attachment; filename="alertas.xlsx"' },
        });
        vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response));

        const { blob, filename } = await httpClient.getBlob("/api/x");
        expect(filename).toBe("alertas.xlsx");
        expect(await blob.text()).toBe("xlsx");
    });
});

describe("httpClient con sesión", () => {
    const session = { accessToken: "tok-1", expiresAt: "2099-01-01T00:00:00Z", username: "admin" };
    let win: EventTarget;

    beforeEach(() => {
        vi.stubGlobal("localStorage", memoryStorage());
        win = new EventTarget();
        vi.stubGlobal("window", win);
    });

    afterEach(() => {
        tokenStore.clear();
    });

    it("manda el token como Bearer si hay sesión", async () => {
        tokenStore.set(session);
        const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, {}));
        vi.stubGlobal("fetch", fetchMock);

        await httpClient.get("/api/x");

        expect(fetchMock.mock.calls[0][1].headers).toMatchObject({ Authorization: "Bearer tok-1" });
    });

    it("sin sesión no manda Authorization", async () => {
        const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, {}));
        vi.stubGlobal("fetch", fetchMock);

        await httpClient.get("/api/x");

        expect(fetchMock.mock.calls[0][1].headers).not.toHaveProperty("Authorization");
    });

    it("un 401 borra la sesión y avisa que venció", async () => {
        tokenStore.set(session);
        const expired = vi.fn();
        win.addEventListener(SESSION_EXPIRED_EVENT, expired);
        vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse(401, { code: "UNAUTHORIZED", message: "Sesión vencida" })));

        await expect(httpClient.get("/api/x")).rejects.toMatchObject({ status: 401, code: "UNAUTHORIZED" });

        expect(tokenStore.get()).toBeNull();
        expect(expired).toHaveBeenCalledOnce();
    });

    it("el 401 del login es contraseña incorrecta: no toca la sesión", async () => {
        tokenStore.set(session);
        const expired = vi.fn();
        win.addEventListener(SESSION_EXPIRED_EVENT, expired);
        vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse(401, { code: "INVALID_CREDENTIALS", message: "x" })));

        await expect(httpClient.post("/api/auth/login", {})).rejects.toMatchObject({ code: "INVALID_CREDENTIALS" });

        expect(tokenStore.get()).toEqual(session);
        expect(expired).not.toHaveBeenCalled();
    });

    it("otros errores no tocan la sesión", async () => {
        tokenStore.set(session);
        vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse(403, { code: "FORBIDDEN", message: "x" })));

        await expect(httpClient.get("/api/x")).rejects.toMatchObject({ status: 403 });

        expect(tokenStore.get()).toEqual(session);
    });
});

describe("Retry-After", () => {
    it("expone los segundos de espera en el error", async () => {
        const response = new Response(JSON.stringify({ code: "TOO_MANY_ATTEMPTS", message: "x" }), {
            status: 429,
            headers: { "Retry-After": "600" },
        });
        vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response));

        await expect(httpClient.post("/api/auth/login", {})).rejects.toMatchObject({ status: 429, retryAfterSeconds: 600 });
    });

    it("es null si no viene o no es un número", async () => {
        vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse(429, { code: "TOO_MANY_ATTEMPTS", message: "x" })));

        await expect(httpClient.post("/api/auth/login", {})).rejects.toMatchObject({ retryAfterSeconds: null });
    });
});

describe("blockBackendRequests", () => {
    it("rechaza las peticiones sin llegar a la red", async () => {
        // Módulo nuevo: el bloqueo es global y no debe afectar a las demás pruebas.
        vi.resetModules();
        const http = await import("./httpClient");
        const fetchMock = vi.fn();
        vi.stubGlobal("fetch", fetchMock);
        vi.spyOn(console, "error").mockImplementation(() => {});

        http.blockBackendRequests("La demo no usa el backend.");

        await expect(http.httpClient.get("/api/x")).rejects.toMatchObject({
            code: http.BACKEND_BLOCKED_CODE,
            message: "La demo no usa el backend.",
        });
        expect(fetchMock).not.toHaveBeenCalled();
    });
});
