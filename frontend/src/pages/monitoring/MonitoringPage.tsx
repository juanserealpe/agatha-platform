import { useState } from "react";
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

function Placeholder({ label }: { label: string }) {
    return (
        <div className="border border-dashed border-gray-300 rounded-lg p-6 text-sm text-gray-400 font-data-mono">
            {label}
        </div>
    );
}

export function MonitoringPage() {
    const { data: devices = [], isLoading, error } = useDevices();
    const [selectedDevice, setSelectedDevice] = useState<Device | null>(null);
    const activeDevice = selectedDevice ?? devices[0] ?? null;

    return (
        <>
            <div className="flex flex-col lg:flex-row gap-6">
                <div className="lg:w-[65%]">
                    <PipelineMap sensores={SENSORES_MOCK} />
                </div>
                <div className="lg:w-[35%]">
                    {isLoading ? (
                        <Placeholder label="Cargando sensores..." />
                    ) : error ? (
                        <Placeholder label="Error al cargar sensores" />
                    ) : (
                        <SensorsList
                            devices={devices}
                            selectedDeviceId={activeDevice?.id}
                            onSensorClick={setSelectedDevice}
                        />
                    )}
                </div>
            </div>

            {activeDevice ? (
                <TelemetryPanel deviceId={activeDevice.id} />
            ) : (
                <Placeholder label="Selecciona un sensor para ver sus gráficas" />
            )}
        </>
    );
}
