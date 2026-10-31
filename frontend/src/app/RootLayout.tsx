import { Outlet } from "react-router";
import { Header } from "@/components/layout/Header";
import { NavTabs } from "@/components/layout/NavTabs";
import { AlertDetailHost, RealtimeStatus, useAlertStream } from "@/features/alerts";

export function RootLayout() {
    // Una sola conexión de tiempo real para toda la app, en cualquier página.
    useAlertStream();

    // Márgenes contenidos y ancho hasta 1600 px: en monitores grandes el espacio va al mapa, no a los costados.
    return (
        <div className="min-h-screen bg-tgi-fondo p-3 md:p-4">
            <div className="mx-auto flex max-w-[1600px] flex-col gap-4 rounded-2xl bg-white p-4 shadow-sm lg:p-5">
                <Header connectionStatus={<RealtimeStatus />} />
                <NavTabs />
                <main className="flex flex-col gap-4">
                    <Outlet />
                </main>
            </div>
            <AlertDetailHost />
        </div>
    );
}
