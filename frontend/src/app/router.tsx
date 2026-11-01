import { createBrowserRouter, Navigate } from "react-router";
import { RootLayout } from "./RootLayout";
import { ROUTES } from "./routes";
import { MonitoringPage } from "@/pages/monitoring/MonitoringPage";
import { HistoryPage } from "@/pages/history/HistoryPage";

export interface AppRouterOptions {
    /** `/demo` en modo demo: las mismas rutas quedan debajo (`/demo/monitoreo`) y los enlaces no salen de ahí. */
    basename?: string;
}

export function createAppRouter({ basename }: AppRouterOptions = {}) {
    return createBrowserRouter(
        [
            {
                path: "/",
                element: <RootLayout />,
                children: [
                    { index: true, element: <Navigate to={ROUTES.monitoring} replace /> },
                    { path: ROUTES.monitoring, element: <MonitoringPage /> },
                    { path: ROUTES.history, element: <HistoryPage /> },
                    { path: "*", element: <Navigate to={ROUTES.monitoring} replace /> },
                ],
            },
        ],
        { basename }
    );
}
