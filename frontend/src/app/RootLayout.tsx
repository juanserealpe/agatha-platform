import { Outlet } from "react-router";
import { Header } from "@/components/layout/Header";
import { NavTabs } from "@/components/layout/NavTabs";
import { AlertDetailHost, RealtimeStatus, useAlertStream } from "@/features/alerts";
import { LogoutButton } from "@/features/auth";
import { verifySession } from "@/services/auth";
import { useAppMode } from "./appModeContext";

// La demo no tiene sesión ni backend: su stream simulado reconecta siempre.
const DEMO_STREAM_OPTIONS = {};
const REAL_STREAM_OPTIONS = { shouldReconnect: verifySession };

export function RootLayout() {
    const { demo, overlay } = useAppMode();
    // Una sola conexión de tiempo real para toda la app, en cualquier página. Si el navegador la cierra
    // (p. ej. 401 por token vencido), la app real verifica la sesión antes de reconectar.
    useAlertStream(demo ? DEMO_STREAM_OPTIONS : REAL_STREAM_OPTIONS);

    // Márgenes contenidos y ancho hasta 1600 px: en monitores grandes el espacio va al mapa, no a los costados.
    return (
        <div className="min-h-screen bg-tgi-fondo p-3 md:p-4">
            {overlay}
            <div className="mx-auto flex max-w-[1600px] flex-col gap-4 rounded-2xl bg-white p-4 shadow-sm lg:p-5">
                <Header connectionStatus={<RealtimeStatus />} sessionControl={demo ? null : <LogoutButton />} />
                <NavTabs />
                <main className="flex flex-col gap-4">
                    <Outlet />
                </main>
            </div>
            <AlertDetailHost />
        </div>
    );
}
