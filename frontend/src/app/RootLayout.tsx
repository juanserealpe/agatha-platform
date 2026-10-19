import { Outlet } from "react-router";
import { Header } from "@/components/layout/Header";
import { NavTabs } from "@/components/layout/NavTabs";
import { AlertDetailHost } from "@/features/alerts";

export function RootLayout() {
    // Márgenes contenidos y ancho hasta 1600 px: en monitores grandes el espacio va al mapa, no a los costados.
    return (
        <div className="min-h-screen bg-gray-100 p-3 md:p-4">
            <div className="mx-auto flex max-w-[1600px] flex-col gap-4 rounded-2xl bg-white p-4 shadow-sm lg:p-5">
                <Header />
                <NavTabs />
                <main className="flex flex-col gap-4">
                    <Outlet />
                </main>
            </div>
            <AlertDetailHost />
        </div>
    );
}
