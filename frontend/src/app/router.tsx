import { createBrowserRouter, Navigate, type RouteObject } from "react-router";
import { RequireSession } from "./RequireSession";
import { RootLayout } from "./RootLayout";
import { ROUTES } from "./routes";
import { LoginPage } from "@/pages/login/LoginPage";
import { MonitoringPage } from "@/pages/monitoring/MonitoringPage";
import { HistoryPage } from "@/pages/history/HistoryPage";

export interface AppRouterOptions {
    /** `/demo` en modo demo: las mismas rutas quedan debajo (`/demo/monitoreo`) y los enlaces no salen de ahí. */
    basename?: string;
    /** false en la demo: no hay backend real ni login. */
    requireSession?: boolean;
}

/** Rutas de la app; aparte de `createAppRouter` para poder probarlas sin un navegador. */
export function createAppRoutes({ requireSession = true }: Pick<AppRouterOptions, "requireSession"> = {}): RouteObject[] {
    const layout = requireSession ? (
        <RequireSession>
            <RootLayout />
        </RequireSession>
    ) : (
        <RootLayout />
    );
    return [
        ...(requireSession ? [{ path: ROUTES.login, element: <LoginPage /> }] : []),
        {
            path: "/",
            element: layout,
            children: [
                { index: true, element: <Navigate to={ROUTES.monitoring} replace /> },
                { path: ROUTES.monitoring, element: <MonitoringPage /> },
                { path: ROUTES.history, element: <HistoryPage /> },
                { path: "*", element: <Navigate to={ROUTES.monitoring} replace /> },
            ],
        },
    ];
}

export function createAppRouter({ basename, requireSession = true }: AppRouterOptions = {}) {
    return createBrowserRouter(createAppRoutes({ requireSession }), { basename });
}
