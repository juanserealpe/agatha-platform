import { useId, useMemo, useRef, useState, type KeyboardEvent } from "react";
import { AlertCircle, ArrowRight, BellOff } from "lucide-react";
import { Link } from "react-router";
import { ROUTES } from "@/app/routes";
import type { AlertItem } from "@/services/alertManagement";
import { EmptyState } from "@/shared/components/EmptyState";
import { Panel } from "@/shared/components/Panel";
import { cn } from "@/lib/utils";
import { useActiveAlerts, usePendingAlerts } from "../hooks/useAlerts";
import { useAlertDetailParam } from "../hooks/useAlertDetailParam";
import { activeAlertsByPriority } from "../lib/alertPriority";
import { EMPTY_FILTERS, withHistorySearch } from "../lib/historySearchParams";
import { AlertListItem } from "./AlertListItem";

// El enlace abre el historial ya filtrado por Sin clasificar.
const PENDING_HISTORY_URL = `${ROUTES.history}?${withHistorySearch(new URLSearchParams(), {
    filters: { ...EMPTY_FILTERS, statuses: ["SIN_CLASIFICAR"] },
    page: 1,
})}`;

const FOOTER_LINK_CLASS =
    "flex w-full items-center justify-center gap-1.5 rounded-lg border border-gray-200 py-2 text-xs font-semibold text-gray-700 transition-colors hover:bg-gray-50 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-gray-900";

type PanelTab = "pending" | "active";

const TAB_ORDER: readonly PanelTab[] = ["pending", "active"];
const TAB_LABEL: Record<PanelTab, string> = { pending: "Sin clasificar", active: "Activas" };

interface PanelTabsProps {
    selected: PanelTab;
    counts: Record<PanelTab, number>;
    idPrefix: string;
    onSelect: (tab: PanelTab) => void;
}

/** Pestañas accesibles: Tab entra a la seleccionada y las flechas cambian de pestaña. */
function PanelTabs({ selected, counts, idPrefix, onSelect }: PanelTabsProps) {
    const refs = useRef<Partial<Record<PanelTab, HTMLButtonElement | null>>>({});

    const onKeyDown = (event: KeyboardEvent) => {
        if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") return;
        event.preventDefault();
        const step = event.key === "ArrowRight" ? 1 : -1;
        const next = TAB_ORDER[(TAB_ORDER.indexOf(selected) + step + TAB_ORDER.length) % TAB_ORDER.length];
        onSelect(next);
        refs.current[next]?.focus();
    };

    return (
        <div role="tablist" aria-label="Listas de alertas" className="flex gap-1 rounded-lg bg-gray-100 p-1" onKeyDown={onKeyDown}>
            {TAB_ORDER.map((tab) => {
                const isSelected = tab === selected;
                const count = counts[tab];
                return (
                    <button
                        key={tab}
                        ref={(element) => {
                            refs.current[tab] = element;
                        }}
                        type="button"
                        role="tab"
                        id={`${idPrefix}-${tab}-tab`}
                        aria-selected={isSelected}
                        aria-controls={`${idPrefix}-panel`}
                        tabIndex={isSelected ? 0 : -1}
                        onClick={() => onSelect(tab)}
                        className={cn(
                            "flex flex-1 items-center justify-center gap-1.5 rounded-md px-2 py-1.5 text-xs font-semibold transition-colors",
                            "focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-gray-900",
                            isSelected ? "bg-white text-gray-900 shadow-xs" : "text-gray-600 hover:text-gray-900"
                        )}
                    >
                        {TAB_LABEL[tab]}
                        {count > 0 && (
                            <span
                                className={cn(
                                    "rounded-full px-1.5 font-data-mono text-[11px]",
                                    // El rojo queda para lo que falta clasificar.
                                    tab === "pending" ? "bg-red-50 text-red-700 ring-1 ring-red-200" : "bg-gray-200 text-gray-700"
                                )}
                            >
                                {count}
                            </span>
                        )}
                    </button>
                );
            })}
        </div>
    );
}

interface AlertListProps {
    alerts: AlertItem[];
    isLoading: boolean;
    error: unknown;
    emptyTitle: string;
    detail: "state" | "management";
    onSelect: ReturnType<typeof useAlertDetailParam>["open"];
}

function AlertList({ alerts, isLoading, error, emptyTitle, detail, onSelect }: AlertListProps) {
    if (isLoading) return <EmptyState title="Cargando alertas…" />;
    if (error) {
        return <EmptyState Icon={AlertCircle} title="No se pudieron cargar las alertas" description="Reintentando automáticamente." />;
    }
    if (alerts.length === 0) return <EmptyState Icon={BellOff} title={emptyTitle} />;

    return (
        <ul className="flex flex-col gap-2">
            {alerts.map((alert) => (
                <li key={alert.id}>
                    <AlertListItem alert={alert} detail={detail} onSelect={onSelect} />
                </li>
            ))}
        </ul>
    );
}

interface RecentAlertsPanelProps {
    className?: string;
}

/**
 * Panel de alertas con dos listas:
 * - Sin clasificar (por defecto): lo que el operador tiene que atender.
 * - Activas: todas las alertas en curso, clasificadas o no, de la más urgente a la menos urgente. Una alerta
 *   clasificada puede seguir subiendo de nivel; aquí sigue visible hasta que finaliza.
 */
export function RecentAlertsPanel({ className }: RecentAlertsPanelProps) {
    const [tab, setTab] = useState<PanelTab>("pending");
    const idPrefix = useId();
    const pending = usePendingAlerts();
    const active = useActiveAlerts();
    const activeAlerts = useMemo(() => activeAlertsByPriority(active.data), [active.data]);
    const pendingCount = pending.data?.total ?? 0;
    const { open: openAlertDetail } = useAlertDetailParam();

    return (
        <Panel
            title="Alertas"
            toolbar={
                <PanelTabs
                    selected={tab}
                    counts={{ pending: pendingCount, active: activeAlerts.length }}
                    idPrefix={idPrefix}
                    onSelect={setTab}
                />
            }
            footer={
                tab === "pending" ? (
                    <Link to={PENDING_HISTORY_URL} className={FOOTER_LINK_CLASS}>
                        Ver todas las pendientes{pendingCount > 0 && ` (${pendingCount})`}
                        <ArrowRight size={14} aria-hidden />
                    </Link>
                ) : (
                    <Link to={ROUTES.history} className={FOOTER_LINK_CLASS}>
                        Ver historial de alertas
                        <ArrowRight size={14} aria-hidden />
                    </Link>
                )
            }
            className={className}
        >
            <div role="tabpanel" id={`${idPrefix}-panel`} aria-labelledby={`${idPrefix}-${tab}-tab`}>
                {tab === "pending" ? (
                    <AlertList
                        alerts={pending.data?.items ?? []}
                        isLoading={pending.isLoading}
                        error={pending.error}
                        emptyTitle="Sin alertas por clasificar"
                        detail="state"
                        onSelect={openAlertDetail}
                    />
                ) : (
                    <AlertList
                        alerts={activeAlerts}
                        isLoading={active.isLoading}
                        error={active.error}
                        emptyTitle="Sin alertas activas"
                        detail="management"
                        onSelect={openAlertDetail}
                    />
                )}
            </div>
        </Panel>
    );
}
