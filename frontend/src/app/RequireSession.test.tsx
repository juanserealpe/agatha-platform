import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { isValidElement, type ReactElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { memoryStorage } from "@/services/auth/testing";
import { tokenStore } from "@/services/auth/tokenStore";
import { RequireSession } from "./RequireSession";
import { RootLayout } from "./RootLayout";
import { createAppRoutes } from "./router";
import { ROUTES } from "./routes";
import { loginRedirectState, redirectAfterLogin } from "./sessionRedirect";

// Las páginas y el layout cargan Leaflet, que necesita `window`: aquí solo importa la tabla de rutas.
vi.mock("./RootLayout", () => ({ RootLayout: () => null }));
vi.mock("@/pages/monitoring/MonitoringPage", () => ({ MonitoringPage: () => null }));
vi.mock("@/pages/history/HistoryPage", () => ({ HistoryPage: () => null }));

const session = { accessToken: "tok-1", expiresAt: "2099-01-01T00:00:00Z", username: "admin" };

function render(path: string) {
    return renderToStaticMarkup(
        <QueryClientProvider client={new QueryClient()}>
            <MemoryRouter initialEntries={[path]}>
                <RequireSession>
                    <p>contenido privado</p>
                </RequireSession>
            </MemoryRouter>
        </QueryClientProvider>
    );
}

beforeEach(() => {
    vi.stubGlobal("localStorage", memoryStorage());
});

afterEach(() => {
    tokenStore.clear();
    vi.unstubAllGlobals();
});

describe("RequireSession", () => {
    it("con sesión muestra la app", () => {
        tokenStore.set(session);
        expect(render(ROUTES.history)).toContain("contenido privado");
    });

    it("sin sesión no muestra nada de la app", () => {
        expect(render(ROUTES.history)).not.toContain("contenido privado");
    });

    it("con la sesión vencida localmente tampoco", () => {
        tokenStore.set({ ...session, expiresAt: "2000-01-01T00:00:00Z" });
        expect(render(ROUTES.history)).not.toContain("contenido privado");
    });
});

describe("redirección del login", () => {
    it("recuerda la página pedida", () => {
        expect(loginRedirectState(ROUTES.history)).toEqual({ from: ROUTES.history });
    });

    it("tras el login vuelve a la página pedida", () => {
        expect(redirectAfterLogin({ from: ROUTES.history })).toBe(ROUTES.history);
    });

    it.each([[null], [undefined], [{}], [{ from: 42 }], [{ from: ROUTES.login }], [{ from: "//evil.example" }], [{ from: "https://evil.example" }]])(
        "si no hay una página válida (%j) va a monitoreo",
        (state) => {
            expect(redirectAfterLogin(state)).toBe(ROUTES.monitoring);
        }
    );
});

describe("createAppRoutes", () => {
    const elementType = (element: unknown) => (isValidElement(element) ? (element as ReactElement).type : null);

    it("por defecto exige sesión y agrega /login fuera del guard", () => {
        const routes = createAppRoutes();
        expect(routes.map((route) => route.path)).toEqual([ROUTES.login, "/"]);
        expect(elementType(routes[1].element)).toBe(RequireSession);
    });

    it("la demo (requireSession: false) no tiene login ni guard", () => {
        const routes = createAppRoutes({ requireSession: false });
        expect(routes.map((route) => route.path)).toEqual(["/"]);
        expect(elementType(routes[0].element)).toBe(RootLayout);
    });
});
