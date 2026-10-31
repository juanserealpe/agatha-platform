import { AlertCircle, ArrowRight, BellOff } from "lucide-react";
import { Link } from "react-router";
import { ROUTES } from "@/app/routes";
import { EmptyState } from "@/shared/components/EmptyState";
import { Panel } from "@/shared/components/Panel";
import { usePendingAlerts } from "../hooks/useAlerts";
import { useAlertDetailParam } from "../hooks/useAlertDetailParam";
import { EMPTY_FILTERS, withHistorySearch } from "../lib/historySearchParams";
import { AlertListItem } from "./AlertListItem";

// El enlace abre el historial ya filtrado por Sin clasificar.
const PENDING_HISTORY_URL = `${ROUTES.history}?${withHistorySearch(new URLSearchParams(), {
    filters: { ...EMPTY_FILTERS, statuses: ["SIN_CLASIFICAR"] },
    page: 1,
})}`;

interface RecentAlertsPanelProps {
    className?: string;
}

/** Las alertas Sin clasificar más recientes: lo que el operador tiene que atender. */
export function RecentAlertsPanel({ className }: RecentAlertsPanelProps) {
    const { data, isLoading, error } = usePendingAlerts();
    const alerts = data?.items ?? [];
    const pendingCount = data?.total ?? 0;
    const { open: openAlertDetail } = useAlertDetailParam();

    return (
        <Panel
            title="Alertas sin clasificar"
            badge={
                pendingCount > 0 && (
                    <span className="whitespace-nowrap rounded-full bg-red-50 px-2 py-0.5 font-data-mono text-xs font-semibold text-red-700 ring-1 ring-red-200">
                        {pendingCount} sin clasificar
                    </span>
                )
            }
            footer={
                <Link
                    to={PENDING_HISTORY_URL}
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
                <EmptyState Icon={BellOff} title="Sin alertas por clasificar" />
            ) : (
                <ul className="flex flex-col gap-2">
                    {alerts.map((alert) => (
                        <li key={alert.id}>
                            <AlertListItem alert={alert} onSelect={openAlertDetail} />
                        </li>
                    ))}
                </ul>
            )}
        </Panel>
    );
}
