import { useState } from "react";
import { AlertCircle, BellOff } from "lucide-react";
import { EmptyState } from "@/shared/components/EmptyState";
import { Panel } from "@/shared/components/Panel";
import { SensorsList, useDevices } from "@/features/dashboard";
import { PipelineMap } from "@/features/map";
import { TelemetryPanel } from "@/features/telemetry";
import type { Device } from "@/services/dashboard/dashboard.types";
import type { Sensor } from "@/services/types";

// TODO(fase 9): sustituir por la configuración fija del piloto enlazada al estado real del dispositivo.
const SENSORES_MOCK: Sensor[] = [
    {
        id: "s1", pk: "PK 34+470", nombre: "Punto 1", tipoTuberia: "flexible",
        nivelAlerta: "roja", tipoAlerta: "movimiento_terreno",
        descripcionEvento: "El tubo se movió 5 cm de su posición original",
        confianzaDeteccion: 96, aceleracion: "alta", esDatoReal: true,
        bateria: 78, senalCelular: 3,
        ultimaComunicacion: new Date(Date.now() - 5 * 60000).toISOString(),
        coordenadas: { lat: 6.032335, lng: -73.561075 },
    },
    {
        id: "s2", pk: "PK 38+174", nombre: "Punto 2", tipoTuberia: "metalico",
        nivelAlerta: "verde", bateria: 85, senalCelular: 4,
        ultimaComunicacion: new Date().toISOString(),
        coordenadas: { lat: 6.030825, lng: -73.537625 },
    },
    {
        id: "s3", pk: "PK 39+268", nombre: "Punto 3", tipoTuberia: "metalico",
        nivelAlerta: "verde", bateria: 90, senalCelular: 4,
        ultimaComunicacion: new Date().toISOString(),
        coordenadas: { lat: 6.036492, lng: -73.531222 },
    },
];

// Apilados (móvil/tablet) cada panel tiene un tope; en escritorio se reparten el alto de la fila.
const SIDE_PANEL_CLASS = "max-h-[360px] lg:max-h-none lg:flex-1";
const COUNT_BADGE_CLASS = "rounded-full bg-gray-100 px-2 py-0.5 font-data-mono text-xs text-gray-600";

export function MonitoringPage() {
    const { data: devices = [], isLoading, error } = useDevices();
    const [selectedDevice, setSelectedDevice] = useState<Device | null>(null);
    const activeDevice = selectedDevice ?? devices[0] ?? null;

    return (
        <>
            {/* En escritorio la fila ocupa el alto de la ventana bajo el header: el mapa la llena
                y los paneles de la derecha se reparten ese alto con scroll interno, sin estirar la fila. */}
            <div className="grid grid-cols-1 gap-6 lg:h-[clamp(560px,calc(100vh-240px),880px)] lg:grid-cols-[minmax(0,1fr)_340px]">
                <div className="h-[60vh] min-h-[420px] lg:h-full lg:min-h-0">
                    <PipelineMap sensores={SENSORES_MOCK} />
                </div>

                <div className="flex min-h-0 flex-col gap-4">
                    <Panel title="Últimas alertas" className={SIDE_PANEL_CLASS}>
                        <EmptyState Icon={BellOff} title="Sin alertas recientes" />
                    </Panel>

                    <Panel
                        title="Sensores"
                        badge={isLoading || error ? undefined : <span className={COUNT_BADGE_CLASS}>{devices.length}</span>}
                        className={SIDE_PANEL_CLASS}
                    >
                        {isLoading ? (
                            <EmptyState title="Cargando sensores…" />
                        ) : error ? (
                            <EmptyState Icon={AlertCircle} title="No se pudieron cargar los sensores" description="Reintentando automáticamente." />
                        ) : (
                            <SensorsList
                                devices={devices}
                                selectedDeviceId={activeDevice?.id}
                                onSensorClick={setSelectedDevice}
                            />
                        )}
                    </Panel>
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
