import { useState } from "react";
import { Marker, Polyline, Popup, Tooltip, useMap, useMapEvent } from "react-leaflet";
import { useAlertDetailParam } from "@/features/alerts";
import { formatPk } from "@/features/sensors";
import { QUEBRADAS, RIOS } from "../data/hydrographyData";
import { CITIES, FLEXIBLE_SEGMENTS, ROUTE_COORDINATES } from "../data/pipelineData";
import { FLEXIBLE_MASK_STYLE, PIPE_CASING_STYLE, PIPE_STYLES, PK_LABELS_MIN_ZOOM, WATER_STYLES } from "../config/mapStyles";
import type { MapNode } from "../hooks/usePilotNodes";
import { cityLabelIcon, nodeIcon, pkLabelIcon } from "../lib/markerIcons";
import { NodePopupContent, NodeTooltipContent } from "./NodeDetails";
import { useClosePopupOnEscape } from "../hooks/useClosePopupOnEscape";

interface PipelineLayersProps {
    nodes: MapNode[];
    onShowTelemetry?: (deviceId: number) => void;
}

export function PipelineLayers({ nodes, onShowTelemetry }: PipelineLayersProps) {
    const map = useMap();
    const { open: openAlertDetail } = useAlertDetailParam();
    useClosePopupOnEscape();
    const [zoom, setZoom] = useState(() => map.getZoom());
    useMapEvent("zoomend", () => setZoom(map.getZoom()));
    const showPkLabels = zoom >= PK_LABELS_MIN_ZOOM;

    return (
        <>
            {RIOS.map((river, i) => (
                <Polyline key={`river-${i}`} positions={river.coords} pathOptions={WATER_STYLES.river.path}>
                    {river.name && <Tooltip sticky>{river.name}</Tooltip>}
                </Polyline>
            ))}
            {QUEBRADAS.map((stream, i) => (
                <Polyline key={`stream-${i}`} positions={stream.coords} pathOptions={WATER_STYLES.stream.path}>
                    {stream.name && <Tooltip sticky>{stream.name}</Tooltip>}
                </Polyline>
            ))}

            <Polyline positions={ROUTE_COORDINATES} pathOptions={PIPE_CASING_STYLE} interactive={false} />
            <Polyline positions={ROUTE_COORDINATES} pathOptions={PIPE_STYLES.steel.path} interactive={false} />
            {FLEXIBLE_SEGMENTS.map((segment, i) => (
                <Polyline key={`flex-mask-${i}`} positions={segment} pathOptions={FLEXIBLE_MASK_STYLE} interactive={false} />
            ))}
            {FLEXIBLE_SEGMENTS.map((segment, i) => (
                <Polyline key={`flex-${i}`} positions={segment} pathOptions={PIPE_STYLES.flexible.path} interactive={false} />
            ))}

            {CITIES.map((city) => (
                <Marker key={city.name} position={[city.lat, city.lng]} icon={cityLabelIcon(city.name)} interactive={false} keyboard={false} />
            ))}

            {showPkLabels &&
                nodes.map((node) => (
                    <Marker
                        key={`${node.id}-pk`}
                        position={node.position}
                        icon={pkLabelIcon(formatPk(node.pkMeters))}
                        interactive={false}
                        keyboard={false}
                    />
                ))}

            {nodes.map((node) => (
                // keyboard: los puntos se enfocan con Tab y Enter abre el popup.
                <Marker key={node.id} position={node.position} icon={nodeIcon(node.state)} riseOnHover>
                    <Tooltip direction="top" offset={[0, -12]} opacity={1}>
                        <NodeTooltipContent node={node} />
                    </Tooltip>
                    <Popup offset={[0, -6]} minWidth={240} maxWidth={280}>
                        <NodePopupContent
                            node={node}
                            onOpenAlert={(alertId) => {
                                map.closePopup();
                                openAlertDetail(alertId);
                            }}
                            onShowTelemetry={
                                onShowTelemetry &&
                                ((deviceId) => {
                                    map.closePopup();
                                    onShowTelemetry(deviceId);
                                })
                            }
                        />
                    </Popup>
                </Marker>
            ))}
        </>
    );
}
