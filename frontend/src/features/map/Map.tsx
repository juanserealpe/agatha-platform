import { useState, useCallback, useEffect } from "react";
import {
    MapContainer,
    TileLayer,
    Polyline,
    Marker,
    Tooltip,
    useMapEvent,
    Rectangle,
    useMap,
} from "react-leaflet";
import L from "leaflet";
import "leaflet/dist/leaflet.css";
import type { Sensor } from "@/services/types";
import { ROUTE_COORDINATES, FLEXIBLE_SEGMENTS, CITIES } from "./data/pipelineData";
import { getMarkerColor } from "@/theme/severity";
import { RIOS, QUEBRADAS } from "./data/hydrographyData";

const ESRI_SATELLITE_URL =
    "https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}";
const ESRI_ATTRIBUTION =
    "Tiles &copy; Esri — Source: Esri, Maxar, Earthstar Geographics";

//const CENTER: [number, number] = [5.94, -73.58]; // centro aprox. del corredor
const FULL_BOUNDS = L.latLngBounds(ROUTE_COORDINATES);

function sensorIcon(sensor: Sensor) {
    const color = getMarkerColor(sensor.nivelAlerta);
    return L.divIcon({
        className: "",
        html: `<div style="width:16px;height:16px;border-radius:50%;background:${color};border:2px solid white;box-shadow:0 0 0 2px ${color}55;"></div>`,
        iconSize: [16, 16],
        iconAnchor: [8, 8],
    });
}

function cityLabelIcon(name: string) {
    return L.divIcon({
        className: "",
        html: `<div style="font-family:monospace;font-size:11px;font-weight:600;color:white;text-shadow:0 1px 3px rgba(0,0,0,0.9);white-space:nowrap;">${name}</div>`,
        iconSize: [0, 0],
    });
}

interface PipelineLayersProps {
    sensores: Sensor[];
    onSensorClick?: (sensor: Sensor) => void;
}

function PipelineLayers({ sensores, onSensorClick }: PipelineLayersProps) {
    return (
        <>

            {RIOS.map((rio, i) => (
                <Polyline key={`rio-${i}`} positions={rio.coords} pathOptions={{ color: "#38bdf8", weight: 2.5, opacity: 0.75 }}>
                    {rio.name && <Tooltip sticky>{rio.name}</Tooltip>}
                </Polyline>
            ))}
            {QUEBRADAS.map((q, i) => (
                <Polyline key={`quebrada-${i}`} positions={q.coords} pathOptions={{ color: "#7dd3fc", weight: 1.2, opacity: 0.6 }}>
                    {q.name && <Tooltip sticky>{q.name}</Tooltip>}
                </Polyline>
            ))}
            <Polyline positions={ROUTE_COORDINATES} pathOptions={{ color: "#fde047", weight: 4, opacity: 0.95 }} />
            {FLEXIBLE_SEGMENTS.map((seg, i) => (
                <Polyline key={i} positions={seg} pathOptions={{ color: "#f97316", weight: 5, opacity: 0.95 }} />
            ))}
            {CITIES.map((city) => (
                <Marker key={city.name} position={[city.lat, city.lng]} icon={cityLabelIcon(city.name)} />
            ))}
            {sensores.map((sensor) => (
                <Marker
                    key={sensor.id}
                    position={[sensor.coordenadas.lat, sensor.coordenadas.lng]}
                    icon={sensorIcon(sensor)}
                    eventHandlers={{ click: () => onSensorClick?.(sensor) }}
                >
                    <Tooltip
                        permanent={true}
                        direction="top"
                        offset={[0, -10]}
                        className="pk-label"
                        interactive={false}
                    >
                        {sensor.pk}
                    </Tooltip>
                </Marker>
            ))}
        </>
    );
}

// --- Minimapa: patrón oficial de react-leaflet, dos MapContainer sincronizados ---
function MinimapViewport({ parentMap, sensores }: { parentMap: L.Map; sensores: Sensor[] }) {
    //const minimap = useMap();
    const [bounds, setBounds] = useState(parentMap.getBounds());
    const [hoverLatLng, setHoverLatLng] = useState<L.LatLng | null>(null);

    const onClick = useCallback(
        (e: L.LeafletMouseEvent) => parentMap.flyTo(e.latlng, 13.7),
        [parentMap]
    );
    useMapEvent("click", onClick);
    useMapEvent("mousemove", (e) => setHoverLatLng(e.latlng));
    useMapEvent("mouseout", () => setHoverLatLng(null));

    useEffect(() => {
        const onChange = () => setBounds(parentMap.getBounds());
        parentMap.on("move zoom", onChange);
        return () => {
            parentMap.off("move zoom", onChange);
        };
    }, [parentMap]);

    // Tamaño aproximado del recuadro de "vista previa" al hacer clic (~4km de lado)
    // Calcula los bounds exactos que tendrá el mapa principal al hacer zoom a nivel 15 en ese punto
    function previewBounds(center: L.LatLng) {
        const targetZoom = 13.7; // Nivel de zoom usado en parentMap.flyTo(..., 15)

        // Obtenemos el tamaño actual en píxeles del mapa principal
        const size = parentMap.getSize();

        // Proyectamos el centro deseado a píxeles en el zoom objetivo
        const centerPoint = parentMap.project(center, targetZoom);

        // Calculamos las esquinas superior izquierda e inferior derecha en píxeles
        const halfWidth = size.x / 2;
        const halfHeight = size.y / 2;

        const southWestPoint = L.point(centerPoint.x - halfWidth, centerPoint.y + halfHeight);
        const northEastPoint = L.point(centerPoint.x + halfWidth, centerPoint.y - halfHeight);

        // Reconvertimos los puntos de píxeles a coordenadas LatLng
        const southWest = parentMap.unproject(southWestPoint, targetZoom);
        const northEast = parentMap.unproject(northEastPoint, targetZoom);

        return L.latLngBounds(southWest, northEast);
    }

    return (
        <>
            <Polyline positions={ROUTE_COORDINATES} pathOptions={{ color: "#fde047", weight: 3, opacity: 1 }} />
            {FLEXIBLE_SEGMENTS.map((seg, i) => (
                <Polyline key={i} positions={seg} pathOptions={{ color: "#f97316", weight: 3 }} />
            ))}
            {sensores.map((s) => (
                <Marker
                    key={s.id}
                    position={[s.coordenadas.lat, s.coordenadas.lng]}
                    icon={L.divIcon({
                        className: "",
                        html: `<div style="width:8px;height:8px;border-radius:50%;background:${getMarkerColor(s.nivelAlerta)};border:1px solid white;"></div>`,
                        iconSize: [8, 8],
                        iconAnchor: [4, 4],
                    })}
                />
            ))}

            {/* Área que el mapa grande está mostrando ahora mismo */}
            <Rectangle bounds={bounds} pathOptions={{ color: "#38bdf8", weight: 2, fillOpacity: 0.08 }} />

            {/* Recuadro que sigue el cursor: indica a dónde saltarías si haces clic */}
            {hoverLatLng && (
                <Rectangle
                    bounds={previewBounds(hoverLatLng)}
                    pathOptions={{ color: "#ffffff", weight: 1.5, dashArray: "4 3", fillOpacity: 0.15, fillColor: "#ffffff" }}
                    interactive={false}
                />
            )}
        </>
    );
}

function MiniMap({ parentMap, sensores }: { parentMap: L.Map; sensores: Sensor[] }) {
    return (
        <div className="absolute top-3 left-3 z-[1000] w-48 h-48 rounded-lg overflow-hidden border-2 border-white shadow-lg cursor-crosshair">
            <MapContainer bounds={FULL_BOUNDS} style={{ height: "100%", width: "100%" }} zoomControl={false} attributionControl={false} dragging={false} scrollWheelZoom={false} doubleClickZoom={false}>
                <TileLayer url={ESRI_SATELLITE_URL} attribution={ESRI_ATTRIBUTION} />
                <TileLayer url="https://server.arcgisonline.com/ArcGIS/rest/services/Reference/World_Boundaries_and_Places/MapServer/tile/{z}/{y}/{x}" />
                <FitBoundsOnMount bounds={FULL_BOUNDS} />
                <MinimapViewport parentMap={parentMap} sensores={sensores} />
            </MapContainer>
        </div>
    );
}

interface PipelineMapProps {
    sensores: Sensor[];
    onSensorClick?: (sensor: Sensor) => void;
}

function FitBoundsOnMount({ bounds }: { bounds: L.LatLngBounds }) {
    const map = useMap();
    useEffect(() => {
        map.fitBounds(bounds, { padding: [40, 40] });
    }, [map, bounds]);
    return null;
}

export function PipelineMap({ sensores, onSensorClick }: PipelineMapProps) {
    const [mainMap, setMainMap] = useState<L.Map | null>(null);

    return (
        <div className="relative w-full h-[500px] rounded-xl overflow-hidden border border-border">
            <MapContainer
                bounds={FULL_BOUNDS}
                style={{ height: "100%", width: "100%" }}
                ref={setMainMap}
            >
                <TileLayer url={ESRI_SATELLITE_URL} attribution={ESRI_ATTRIBUTION} />
                <TileLayer url="https://server.arcgisonline.com/ArcGIS/rest/services/Reference/World_Boundaries_and_Places/MapServer/tile/{z}/{y}/{x}" />
                <FitBoundsOnMount bounds={FULL_BOUNDS} />
                <PipelineLayers sensores={sensores} onSensorClick={onSensorClick} />
            </MapContainer>
            {mainMap && <MiniMap parentMap={mainMap} sensores={sensores} />}

            <div className="absolute bottom-3 right-3 z-[1000] bg-white/90 backdrop-blur rounded-lg p-2.5 text-xs font-data-mono shadow-md flex flex-col gap-1">
                <div className="flex items-center gap-2"><span className="w-4 h-1 bg-[#fde047] inline-block rounded" /> Tubo Acero</div>
                <div className="flex items-center gap-2"><span className="w-4 h-1 bg-[#f97316] inline-block rounded" /> Tubo Flexible</div>
                <div className="flex items-center gap-2"><span className="w-4 h-1 bg-[#38bdf8] inline-block rounded" /> Río</div>
                <div className="flex items-center gap-2"><span className="w-4 h-1 bg-[#7dd3fc] inline-block rounded" /> Quebrada</div>
            </div>
        </div>
    );
}