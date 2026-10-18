import { Outlet } from "react-router";
import { Header } from "@/components/layout/Header";
import { NavTabs } from "@/components/layout/NavTabs";

export function RootLayout() {
    return (
        <div className="p-4 md:p-8 bg-gray-100 min-h-screen">
            <div className="max-w-[1200px] mx-auto bg-white rounded-2xl p-6 flex flex-col gap-6 shadow-sm">
                <Header />
                <NavTabs />
                <main className="flex flex-col gap-6">
                    <Outlet />
                </main>
            </div>
        </div>
    );
}
