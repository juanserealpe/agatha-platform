import { useState, type ReactNode } from "react";
import { MapContainer, TileLayer } from "react-leaflet";
import type L from "leaflet";
import { TILE_LAYERS } from "../config/mapStyles";
import type { MapNode } from "../hooks/usePilotNodes";
import { FULL_BOUNDS } from "../lib/bounds";
import { FitBoundsOnMount, InvalidateSizeOnResize } from "./mapHelpers";
import { MapLegend } from "./MapLegend";
import { MiniMap } from "./MiniMap";
import { PipelineLayers } from "./PipelineLayers";

interface PipelineMapProps {
    nodes: MapNode[];
    /** Acción "Ver telemetría" del popup de un punto. */
    onShowTelemetry?: (deviceId: number) => void;
    /** Contenido flotante arriba a la derecha del mapa (p. ej. avisos), sin tapar el centro. */
    topRightOverlay?: ReactNode;
}

export function PipelineMap({ nodes, onShowTelemetry, topRightOverlay }: PipelineMapProps) {
    const [mainMap, setMainMap] = useState<L.Map | null>(null);

    // isolate: los z-index de Leaflet (400–1000) quedan contenidos y no tapan modales ni popovers de la app.
    return (
        <div className="relative isolate h-full w-full overflow-hidden rounded-xl border border-border shadow-xs">
            <MapContainer bounds={FULL_BOUNDS} style={{ height: "100%", width: "100%" }} ref={setMainMap}>
                <TileLayer url={TILE_LAYERS.satellite.url} attribution={TILE_LAYERS.satellite.attribution} />
                <TileLayer url={TILE_LAYERS.places.url} />
                <FitBoundsOnMount />
                <InvalidateSizeOnResize />
                <PipelineLayers nodes={nodes} onShowTelemetry={onShowTelemetry} />
            </MapContainer>

            {topRightOverlay && (
                <div className="pointer-events-none absolute right-3 top-3 z-[1000] flex justify-end [&>*]:pointer-events-auto">
                    {topRightOverlay}
                </div>
            )}
            {mainMap && <MiniMap parentMap={mainMap} nodes={nodes} />}
            <MapLegend />
        </div>
    );
}
