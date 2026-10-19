import { useRef } from "react";
import { EmptyState } from "@/shared/components/EmptyState";
import { NewAlertToast, RecentAlertsPanel } from "@/features/alerts";
import { SensorsPanel, useActiveDevice } from "@/features/sensors";
import { PipelineMap, usePilotNodes } from "@/features/map";
import { TelemetryPanel } from "@/features/telemetry";

// Apilados (móvil/tablet) cada panel tiene un tope. En escritorio, alertas (lo urgente y lo que más crece)
// toma el alto libre; sensores (pocos en el piloto) ocupa solo lo que necesita, hasta un 45 % de la columna.
const ALERTS_PANEL_CLASS = "max-h-[420px] lg:max-h-none lg:flex-1";
const SENSORS_PANEL_CLASS = "max-h-[360px] lg:max-h-[45%] lg:flex-none";

export function MonitoringPage() {
    const { activeDevice, selectDevice } = useActiveDevice();
    const nodes = usePilotNodes();
    const telemetryRef = useRef<HTMLElement>(null);

    function showTelemetry(deviceId: number) {
        selectDevice(deviceId);
        telemetryRef.current?.scrollIntoView({ behavior: "smooth", block: "start" });
    }

    return (
        <>
            {/* En escritorio la fila ocupa el alto de la ventana bajo el header (sin pasarse, para no obligar a
                hacer scroll de página sobre el mapa): el mapa la llena y los paneles usan scroll interno. */}
            <div className="grid grid-cols-1 gap-4 lg:h-[clamp(520px,calc(100vh-180px),960px)] lg:grid-cols-[minmax(0,1fr)_360px]">
                <div className="h-[60vh] min-h-[420px] lg:h-full lg:min-h-0">
                    <PipelineMap nodes={nodes} onShowTelemetry={showTelemetry} topRightOverlay={<NewAlertToast />} />
                </div>

                <div className="flex min-h-0 flex-col gap-4">
                    <RecentAlertsPanel className={ALERTS_PANEL_CLASS} />

                    <SensorsPanel
                        selectedDeviceId={activeDevice?.id}
                        onSelect={selectDevice}
                        className={SENSORS_PANEL_CLASS}
                    />
                </div>
            </div>

            <section ref={telemetryRef} aria-label="Telemetría del sensor" className="scroll-mt-4">
                {activeDevice ? (
                    <TelemetryPanel deviceId={activeDevice.id} />
                ) : (
                    <EmptyState title="Selecciona un sensor para ver sus gráficas" />
                )}
            </section>
        </>
    );
}
