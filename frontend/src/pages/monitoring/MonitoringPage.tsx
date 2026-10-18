import { EmptyState } from "@/shared/components/EmptyState";
import { RecentAlertsPanel } from "@/features/alerts";
import { SensorsPanel, useActiveDevice } from "@/features/sensors";
import { PipelineMap, usePilotNodes } from "@/features/map";
import { TelemetryPanel } from "@/features/telemetry";

// Apilados (móvil/tablet) cada panel tiene un tope; en escritorio se reparten el alto de la fila.
const SIDE_PANEL_CLASS = "max-h-[360px] lg:max-h-none lg:flex-1";

export function MonitoringPage() {
    const { activeDevice, selectDevice } = useActiveDevice();
    const nodes = usePilotNodes();

    return (
        <>
            {/* En escritorio la fila ocupa el alto de la ventana bajo el header: el mapa la llena
                y los paneles de la derecha se reparten ese alto con scroll interno, sin estirar la fila. */}
            <div className="grid grid-cols-1 gap-6 lg:h-[clamp(560px,calc(100vh-240px),880px)] lg:grid-cols-[minmax(0,1fr)_340px]">
                <div className="h-[60vh] min-h-[420px] lg:h-full lg:min-h-0">
                    <PipelineMap nodes={nodes} />
                </div>

                <div className="flex min-h-0 flex-col gap-4">
                    <RecentAlertsPanel className={SIDE_PANEL_CLASS} />

                    <SensorsPanel
                        selectedDeviceId={activeDevice?.id}
                        onSelect={selectDevice}
                        className={SIDE_PANEL_CLASS}
                    />
                </div>
            </div>

            {activeDevice ? (
                <TelemetryPanel deviceId={activeDevice.id} />
            ) : (
                <EmptyState title="Selecciona un sensor para ver sus gráficas" />
            )}
        </>
    );
}
