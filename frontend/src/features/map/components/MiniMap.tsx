import { useCallback, useEffect, useState } from "react";
import { ChevronDown, Map as MapIcon } from "lucide-react";
import { MapContainer, Marker, Polyline, Rectangle, TileLayer, useMapEvent } from "react-leaflet";
import L from "leaflet";
import { ROUTE_COORDINATES } from "../data/pipelineData";
import { PIPE_CASING_STYLE, PIPE_STYLES, PK_LABELS_MIN_ZOOM, TILE_LAYERS } from "../config/mapStyles";
import type { MapNode } from "../hooks/usePilotNodes";
import { miniNodeIcon } from "../lib/markerIcons";
import { FULL_BOUNDS } from "../lib/bounds";
import { FitBoundsOnMount } from "./mapHelpers";

// Zoom al que salta el mapa principal al hacer clic en el minimapa: el mismo en el que aparecen
// las etiquetas de PK, para que al llegar ya se vean.
const JUMP_ZOOM = PK_LABELS_MIN_ZOOM;

// Margen pequeño: en un recuadro de 192 px, el margen del mapa grande dejaría el tramo diminuto.
const MINIMAP_PADDING_PX = 8;

// El recuadro de vista previa lleva un halo oscuro debajo para leerse sobre cualquier zona del satélite.
const PREVIEW_HALO = { color: "#0f172a", weight: 4, opacity: 0.6, fill: false };
const PREVIEW_LINE = { color: "#ffffff", weight: 2, dashArray: "5 3", fillColor: "#ffffff", fillOpacity: 0.2 };

interface MiniMapProps {
    parentMap: L.Map;
    nodes: MapNode[];
}

/** Bounds que mostraría el mapa principal centrado en `center` al zoom de salto. */
function previewBounds(parentMap: L.Map, center: L.LatLng): L.LatLngBounds {
    const size = parentMap.getSize();
    const centerPoint = parentMap.project(center, JUMP_ZOOM);
    const southWest = L.point(centerPoint.x - size.x / 2, centerPoint.y + size.y / 2);
    const northEast = L.point(centerPoint.x + size.x / 2, centerPoint.y - size.y / 2);
    return L.latLngBounds(parentMap.unproject(southWest, JUMP_ZOOM), parentMap.unproject(northEast, JUMP_ZOOM));
}

function MinimapViewport({ parentMap, nodes }: MiniMapProps) {
    const [bounds, setBounds] = useState(parentMap.getBounds());
    const [hoverLatLng, setHoverLatLng] = useState<L.LatLng | null>(null);

    const onClick = useCallback((e: L.LeafletMouseEvent) => parentMap.flyTo(e.latlng, JUMP_ZOOM), [parentMap]);
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

    return (
        <>
            <Polyline positions={ROUTE_COORDINATES} pathOptions={{ ...PIPE_CASING_STYLE, weight: 4 }} />
            <Polyline positions={ROUTE_COORDINATES} pathOptions={{ ...PIPE_STYLES.steel.path, weight: 2 }} />
            {nodes.map((node) => (
                <Marker key={node.id} position={node.position} icon={miniNodeIcon(node.state)} interactive={false} />
            ))}

            {/* Área que muestra ahora el mapa principal */}
            <Rectangle bounds={bounds} pathOptions={{ color: "#38bdf8", weight: 2, fillOpacity: 0.1 }} interactive={false} />

            {/* Hacia dónde saltaría el mapa principal con un clic: es exactamente el área que se verá */}
            {hoverLatLng && (
                <>
                    <Rectangle bounds={previewBounds(parentMap, hoverLatLng)} pathOptions={PREVIEW_HALO} interactive={false} />
                    <Rectangle bounds={previewBounds(parentMap, hoverLatLng)} pathOptions={PREVIEW_LINE} interactive={false} />
                </>
            )}
        </>
    );
}

export function MiniMap({ parentMap, nodes }: MiniMapProps) {
    const [isExpanded, setIsExpanded] = useState(true);

    // Marco blanco grueso + sombra fuerte: separa el minimapa del satélite del mapa grande.
    return (
        <div className="absolute bottom-3 left-3 z-[1000] overflow-hidden rounded-lg border-[3px] border-white bg-white shadow-[0_4px_16px_rgba(0,0,0,0.45)] ring-1 ring-black/20">
            <button
                type="button"
                onClick={() => setIsExpanded((value) => !value)}
                aria-expanded={isExpanded}
                className="flex w-full items-center gap-1.5 px-2.5 py-1.5 text-xs font-medium text-gray-700 hover:bg-gray-50"
            >
                <MapIcon size={14} aria-hidden />
                Vista general
                <ChevronDown size={14} className={`ml-auto transition-transform ${isExpanded ? "" : "rotate-180"}`} aria-hidden />
            </button>

            {isExpanded && (
                <div className="h-40 w-48 cursor-crosshair overflow-hidden rounded-md">
                    <MapContainer
                        bounds={FULL_BOUNDS}
                        style={{ height: "100%", width: "100%" }}
                        zoomControl={false}
                        attributionControl={false}
                        dragging={false}
                        scrollWheelZoom={false}
                        doubleClickZoom={false}
                        keyboard={false}
                        // Zoom fraccionario: el tramo llena el recuadro en vez de quedar al zoom entero inferior.
                        zoomSnap={0.1}
                    >
                        <TileLayer url={TILE_LAYERS.satellite.url} />
                        <TileLayer url={TILE_LAYERS.places.url} />
                        <FitBoundsOnMount padding={MINIMAP_PADDING_PX} />
                        <MinimapViewport parentMap={parentMap} nodes={nodes} />
                    </MapContainer>
                </div>
            )}
        </div>
    );
}
