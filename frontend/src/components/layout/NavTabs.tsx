import type { LucideIcon } from "lucide-react";
import { History, LayoutDashboard } from "lucide-react";
import { NavLink } from "react-router";
import { ROUTES } from "@/app/routes";
import { cn } from "@/lib/utils";

interface Tab {
    to: string;
    label: string;
    Icon: LucideIcon;
}

const TABS: Tab[] = [
    { to: ROUTES.monitoring, label: "Centro de Control", Icon: LayoutDashboard },
    { to: ROUTES.history, label: "Historial de Alertas", Icon: History },
];

export function NavTabs() {
    return (
        <nav aria-label="Secciones" className="flex flex-wrap gap-2 border-b border-border pb-4">
            {TABS.map(({ to, label, Icon }) => (
                <NavLink
                    key={to}
                    to={to}
                    className={({ isActive }) =>
                        cn(
                            "flex items-center gap-2 rounded-lg px-4 py-2 text-sm font-medium transition-colors",
                            "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring",
                            isActive
                                ? "bg-tgi-verde-oscuro text-white shadow-sm"
                                : "text-gray-600 hover:bg-gray-100 hover:text-gray-900"
                        )
                    }
                >
                    <Icon size={16} aria-hidden />
                    {label}
                </NavLink>
            ))}
        </nav>
    );
}
