/* eslint-disable react-hooks/set-state-in-effect */
/* eslint-disable @typescript-eslint/no-unused-vars */
import { useEffect, useState } from "react";
import { AppShell } from "@/components/layout/AppShell";
import { FiltersBar, SensorsList } from "@/features/dashboard";
import type { FiltersState, Sensor } from "@/services/types";
import { AlertModal } from "@/features/alerts";
import { AlertBanner } from "@/features/alerts";
import { PipelineMap } from "@/features/map";
import { useDevices } from "@/features/dashboard/hooks/useDevices";
import { useMeasurements } from "@/features/dashboard/hooks/useMeasurements";
import { useDateRangeState } from "@/features/dashboard/hooks/useDateRangeState";
import { MetricsAccordion } from "@/features/dashboard/components/MetricsAccordion";
import { DateRangeControls } from "@/features/dashboard/components/DateRangeControls";
import type { Device } from "@/services/dashboard/dashboard.types";
// TODO: reemplazar por datos reales desde el backend (REST API, sección 8 del documento)
const GASODUCTOS_MOCK = [
  { value: "otero-santana", label: "Otero - Santana" },
];

// Opciones para el DROPDOWN de filtro (no confundir con los sensores reales de abajo)
const SENSOR_FILTER_OPTIONS = [
  { value: "s1", label: "PK 34+470" },
  { value: "s2", label: "PK 38+174" },
  { value: "s3", label: "PK 39+268" },
];

const CRITICIDADES = [
  { value: "roja", label: "Rojo (Crítico)" },
  { value: "amarilla", label: "Amarillo (Alerta)" },
];

// Datos reales de los sensores para la LISTA (SensorsList)
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

function App() {
  //const [selectedSensor, setSelectedSensor] = useState<Sensor | null>(null);
  const [selectedSensor, setSelectedSensor] = useState<Sensor | null>(null);
  const [selectedDevice, setSelectedDevice] = useState<Device | null>(null); // para el panel real
  const [modalOpen, setModalOpen] = useState(false);

  // function handleSensorClick(sensor: Sensor) {
  //   setSelectedSensor(sensor);
  //   setModalOpen(true);
  // }

  const { data: devices = [], isLoading, error } = useDevices();

  const [filters, setFilters] = useState<FiltersState>({
    gasoducto: "",
    sensor: "",
    criticidad: "",
  });

  useEffect(() => {
    if (!selectedDevice && devices.length > 0) {
      setSelectedDevice(devices[0]);
    }
  }, [devices, selectedDevice]);

  const { startDate, endDate, quickRange, selectQuickRange, selectCustomRange } = useDateRangeState();

  const {
    data: measurementsData,
    isLoading: loadingMeasurements,
    error: measurementsError,
  } = useMeasurements(selectedDevice?.id, startDate, endDate);


  return (
    <>
      <AppShell
        alertBanner={
          <AlertBanner
            selectedSensor={selectedSensor}
            onClose={() => setSelectedSensor(null)}
            onConfirmar={(sensor, motivo, obs) => console.log("Confirmar", sensor.id, motivo, obs)}
            onFalsaAlerta={(sensor, motivo, obs) => console.log("Falsa alerta", sensor.id, motivo, obs)}
          />
        }
        filters={
          <FiltersBar
            gasoductos={GASODUCTOS_MOCK}
            sensores={SENSOR_FILTER_OPTIONS}
            criticidades={CRITICIDADES}
            filters={filters}
            onFilterChange={(key, value) =>
              setFilters((prev) => ({ ...prev, [key]: value }))
            }
            onExport={() => console.log("Exportar", filters)}
          />
        }
        map={
          <PipelineMap
            sensores={SENSORES_MOCK}
            onSensorClick={setSelectedSensor}
          />
        }
        recentEvents={
          isLoading ? (
            <Placeholder label="Cargando sensores..." />
          ) : error ? (
            <Placeholder label="Error al cargar sensores" />
          ) : (
            <SensorsList
              devices={devices}
              selectedDeviceId={selectedDevice?.id}
              onSensorClick={setSelectedDevice}
            />
          )
        }
        telemetry={
          !selectedDevice ? (
            <Placeholder label="Selecciona un sensor para ver sus gráficas" />
          ) : (
            <div className="flex flex-col gap-3">
              <DateRangeControls
                activeQuickRange={quickRange}
                onQuickRangeSelect={selectQuickRange}
                onCustomRangeApply={selectCustomRange}
              />
              {loadingMeasurements ? (
                <Placeholder label="Cargando métricas..." />
              ) : measurementsError ? (
                <Placeholder label="Error al cargar métricas" />
              ) : (
                <MetricsAccordion
                  measurements={measurementsData?.measurements ?? []}
                  deviceId={selectedDevice?.id}
                  startDate={startDate}
                  endDate={endDate}
                />
              )}
            </div>
          )
        }
      />
      <AlertModal
        sensor={selectedSensor}
        open={modalOpen}
        onOpenChange={setModalOpen}
        onConfirmar={(sensor, motivo, obs) =>
          console.log("Confirmar", sensor.id, motivo, obs)
        }
        onFalsaAlerta={(sensor, motivo, obs) =>
          console.log("Falsa alerta", sensor.id, motivo, obs)
        }
      />
    </>
  );
}

export default App;