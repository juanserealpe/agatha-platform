import { AlertCircle, ArrowRight, BellOff } from "lucide-react";
import { Link } from "react-router";
import { ROUTES } from "@/app/routes";
import { EmptyState } from "@/shared/components/EmptyState";
import { Panel } from "@/shared/components/Panel";
import { usePendingAlerts, useRecentAlerts } from "../hooks/useAlerts";
import { AlertListItem } from "./AlertListItem";

interface RecentAlertsPanelProps {
    onSelectAlert?: (alertId: string) => void;
    className?: string;
}

export function RecentAlertsPanel({ onSelectAlert, className }: RecentAlertsPanelProps) {
    const { data: alerts = [], isLoading, error } = useRecentAlerts();
    const { data: pending = [] } = usePendingAlerts();
    const pendingCount = pending.length;

    return (
        <Panel
            title="Últimas alertas"
            badge={
                pendingCount > 0 && (
                    <span className="whitespace-nowrap rounded-full bg-red-50 px-2 py-0.5 font-data-mono text-xs font-semibold text-red-700 ring-1 ring-red-200">
                        {pendingCount} sin clasificar
                    </span>
                )
            }
            footer={
                <Link
                    to={ROUTES.history}
                    className="flex w-full items-center justify-center gap-1.5 rounded-lg border border-gray-200 py-2 text-xs font-semibold text-gray-700 transition-colors hover:bg-gray-50 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-gray-900"
                >
                    Ver todas las pendientes{pendingCount > 0 && ` (${pendingCount})`}
                    <ArrowRight size={14} aria-hidden />
                </Link>
            }
            className={className}
        >
            {isLoading ? (
                <EmptyState title="Cargando alertas…" />
            ) : error ? (
                <EmptyState Icon={AlertCircle} title="No se pudieron cargar las alertas" description="Reintentando automáticamente." />
            ) : alerts.length === 0 ? (
                <EmptyState Icon={BellOff} title="Sin alertas recientes" />
            ) : (
                <ul className="flex flex-col gap-2">
                    {alerts.map((alert) => (
                        <li key={alert.id}>
                            <AlertListItem alert={alert} onSelect={onSelectAlert} />
                        </li>
                    ))}
                </ul>
            )}
        </Panel>
    );
}
