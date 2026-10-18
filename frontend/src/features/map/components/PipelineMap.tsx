import { useState } from "react";
import { MapContainer, TileLayer } from "react-leaflet";
import type L from "leaflet";
import "leaflet/dist/leaflet.css";
import { TILE_LAYERS } from "../config/mapStyles";
import type { MapNode } from "../hooks/usePilotNodes";
import { FULL_BOUNDS } from "../lib/bounds";
import { FitBoundsOnMount, InvalidateSizeOnResize } from "./mapHelpers";
import { MapLegend } from "./MapLegend";
import { MiniMap } from "./MiniMap";
import { PipelineLayers } from "./PipelineLayers";

interface PipelineMapProps {
    nodes: MapNode[];
}

export function PipelineMap({ nodes }: PipelineMapProps) {
    const [mainMap, setMainMap] = useState<L.Map | null>(null);

    // isolate: los z-index de Leaflet (400–1000) quedan contenidos y no tapan modales ni popovers de la app.
    return (
        <div className="relative isolate h-full w-full overflow-hidden rounded-xl border border-border shadow-xs">
            <MapContainer bounds={FULL_BOUNDS} style={{ height: "100%", width: "100%" }} ref={setMainMap}>
                <TileLayer url={TILE_LAYERS.satellite.url} attribution={TILE_LAYERS.satellite.attribution} />
                <TileLayer url={TILE_LAYERS.places.url} />
                <FitBoundsOnMount />
                <InvalidateSizeOnResize />
                <PipelineLayers nodes={nodes} />
            </MapContainer>

            {mainMap && <MiniMap parentMap={mainMap} nodes={nodes} />}
            <MapLegend />
        </div>
    );
}
