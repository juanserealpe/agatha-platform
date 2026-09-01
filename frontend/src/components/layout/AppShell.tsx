import type { ReactNode } from "react";
import { Header } from "./Header";

interface AppShellProps {
    filters: ReactNode;
    alertBanner: ReactNode;
    map: ReactNode;
    recentEvents: ReactNode;
    telemetry: ReactNode;
}

export function AppShell({
    filters,
    alertBanner,
    map,
    recentEvents,
    telemetry,
}: AppShellProps) {
    return (
        <div className="p-4 md:p-8 bg-gray-100 min-h-screen relative">
            <div className="max-w-[1200px] mx-auto bg-white rounded-2xl p-6 flex flex-col gap-6 relative z-10 shadow-sm">
                <Header />
                {alertBanner}
                {filters}

                <div className="flex flex-col lg:flex-row gap-6">
                    <div className="lg:w-[65%]">{map}</div>
                    <div className="lg:w-[35%]">{recentEvents}</div>
                </div>

                {telemetry}
            </div>
        </div>
    );
}