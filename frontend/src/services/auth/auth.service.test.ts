import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/services/http";
import { login, loginErrorMessage, logout, verifySession } from "./auth.service";
import { memoryStorage } from "./testing";
import { tokenStore } from "./tokenStore";

function jsonResponse(status: number, body: unknown): Response {
    return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

const session = { accessToken: "tok-1", expiresAt: "2099-01-01T00:00:00Z", username: "admin" };

beforeEach(() => {
    vi.stubGlobal("localStorage", memoryStorage());
    vi.stubGlobal("window", new EventTarget());
});

afterEach(() => {
    tokenStore.clear();
    vi.unstubAllGlobals();
});

describe("login", () => {
    it("manda exactamente usuario y contraseña y guarda la sesión", async () => {
        const fetchMock = vi.fn().mockResolvedValue(
            jsonResponse(200, { ...session, tokenType: "Bearer" })
        );
        vi.stubGlobal("fetch", fetchMock);

        await expect(login("admin", "clave")).resolves.toEqual(session);

        const [url, init] = fetchMock.mock.calls[0];
        expect(url).toBe("/api/auth/login");
        expect(init.method).toBe("POST");
        expect(JSON.parse(init.body)).toEqual({ username: "admin", password: "clave" });
        expect(tokenStore.get()).toEqual(session);
    });

    it("con credenciales incorrectas no guarda nada", async () => {
        vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse(401, { code: "INVALID_CREDENTIALS", message: "x" })));

        await expect(login("admin", "mala")).rejects.toMatchObject({ code: "INVALID_CREDENTIALS" });
        expect(tokenStore.get()).toBeNull();
    });
});

describe("verifySession", () => {
    it("sin token no llama al backend", async () => {
        const fetchMock = vi.fn();
        vi.stubGlobal("fetch", fetchMock);

        await expect(verifySession()).resolves.toBe(false);
        expect(fetchMock).not.toHaveBeenCalled();
    });

    it("es true si /api/auth/me acepta el token", async () => {
        tokenStore.set(session);
        const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, { principal: "admin" }));
        vi.stubGlobal("fetch", fetchMock);

        await expect(verifySession()).resolves.toBe(true);
        expect(fetchMock.mock.calls[0][0]).toBe("/api/auth/me");
        expect(fetchMock.mock.calls[0][1].headers).toMatchObject({ Authorization: "Bearer tok-1" });
    });

    it("con un 401 es false y la sesión queda borrada", async () => {
        tokenStore.set(session);
        vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse(401, { code: "UNAUTHORIZED", message: "x" })));

        await expect(verifySession()).resolves.toBe(false);
        expect(tokenStore.get()).toBeNull();
    });
});

describe("logout", () => {
    it("borra la sesión local", () => {
        tokenStore.set(session);
        logout();
        expect(tokenStore.get()).toBeNull();
    });
});

describe("loginErrorMessage", () => {
    const error = (status: number, code: string, retryAfterSeconds: number | null = null) =>
        new ApiError({ status, code, message: "del backend", retryAfterSeconds });

    it.each([
        [error(400, "INVALID_INPUT"), "Complete usuario y contraseña."],
        [error(401, "INVALID_CREDENTIALS"), "Usuario o contraseña incorrectos."],
        [error(503, "AUTH_NOT_CONFIGURED"), "El inicio de sesión no está disponible. Contacte al administrador."],
        [error(429, "TOO_MANY_ATTEMPTS", 600), "Demasiados intentos. Intente de nuevo en 10 minutos."],
        [error(429, "TOO_MANY_ATTEMPTS", 61), "Demasiados intentos. Intente de nuevo en 2 minutos."],
        [error(429, "TOO_MANY_ATTEMPTS", 30), "Demasiados intentos. Intente de nuevo en 1 minuto."],
        [error(429, "TOO_MANY_ATTEMPTS"), "Demasiados intentos. Intente más tarde."],
    ])("%#: %s", (apiError, expected) => {
        expect(loginErrorMessage(apiError)).toBe(expected);
    });

    it("otro error del backend usa su mensaje; uno desconocido, el genérico", () => {
        expect(loginErrorMessage(error(500, "INTERNAL"))).toBe("del backend");
        expect(loginErrorMessage(new Error("boom"))).toBe("No fue posible iniciar sesión.");
    });
});
