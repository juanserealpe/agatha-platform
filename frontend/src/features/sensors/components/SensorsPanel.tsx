import { useMemo, useState } from "react";
import { AlertCircle, SearchX } from "lucide-react";
import { EmptyState } from "@/shared/components/EmptyState";
import { Panel } from "@/shared/components/Panel";
import { useNow } from "@/shared/hooks/useNow";
import { useDevices } from "../hooks/useDevices";
import { filterDevices } from "../lib/sensorSearch";
import { getDeviceStatus, type SensorAlertIndicator } from "../lib/sensorStatus";
import { SensorListItem } from "./SensorListItem";
import { SensorListSkeleton } from "./SensorListSkeleton";
import { SensorSearch } from "./SensorSearch";

// Con pocos sensores la lista se recorre de un vistazo: el buscador solo ocuparía alto del panel.
const SEARCH_MIN_DEVICES = 7;

interface SensorsPanelProps {
    selectedDeviceId: number | undefined;
    /** Alertas activas por id de dispositivo; los sensores sin entrada se muestran por su comunicación. */
    alerts?: ReadonlyMap<number, SensorAlertIndicator>;
    onSelect: (deviceId: number) => void;
    className?: string;
}

export function SensorsPanel({ selectedDeviceId, alerts, onSelect, className }: SensorsPanelProps) {
    const { data: devices = [], isLoading, error } = useDevices();
    const [query, setQuery] = useState("");
    const now = useNow();

    const showSearch = devices.length >= SEARCH_MIN_DEVICES;
    // Sin buscador a la vista, una búsqueda anterior no debe seguir filtrando.
    const visibleDevices = useMemo(() => (showSearch ? filterDevices(devices, query) : devices), [showSearch, devices, query]);
    const onlineCount = devices.filter((device) => getDeviceStatus(device.lastConnection, now) === "online").length;
    const isReady = !isLoading && !error;

    return (
        <Panel
            title="Sensores"
            badge={
                isReady && (
                    <span className="rounded-full bg-emerald-50 px-2 py-0.5 font-data-mono text-xs text-emerald-700">
                        {onlineCount} en línea
                    </span>
                )
            }
            action={isReady && <span className="text-xs text-gray-500">{devices.length} {devices.length === 1 ? "instalado" : "instalados"}</span>}
            toolbar={isReady && showSearch && <SensorSearch value={query} onChange={setQuery} />}
            className={className}
        >
            {isLoading ? (
                <SensorListSkeleton />
            ) : error ? (
                <EmptyState Icon={AlertCircle} title="No se pudieron cargar los sensores" description="Reintentando automáticamente." />
            ) : devices.length === 0 ? (
                <EmptyState title="No hay sensores registrados" />
            ) : visibleDevices.length === 0 ? (
                <EmptyState Icon={SearchX} title="Ningún sensor coincide" description={`Sin resultados para «${query.trim()}».`} />
            ) : (
                <ul className="flex flex-col gap-2">
                    {visibleDevices.map((device) => (
                        <li key={device.id}>
                            <SensorListItem
                                device={device}
                                status={getDeviceStatus(device.lastConnection, now)}
                                alert={alerts?.get(device.id) ?? null}
                                isSelected={device.id === selectedDeviceId}
                                onSelect={onSelect}
                            />
                        </li>
                    ))}
                </ul>
            )}
        </Panel>
    );
}
