import L from "leaflet";
import { NODE_TONE_STYLES, WARNING_HEX } from "../config/mapStyles";
import type { NodeState } from "./nodeState";

// Los iconos se reutilizan: crear un divIcon nuevo en cada render obliga a Leaflet a redibujar el marcador.
const cache = new Map<string, L.DivIcon>();

function nodeHtml(state: NodeState, size: number): string {
    const color = NODE_TONE_STYLES[state.tone].hex;
    const warningSize = Math.round(size * 0.55);
    const warning = state.hasWarning
        ? `<span style="position:absolute;top:-3px;right:-3px;width:${warningSize}px;height:${warningSize}px;border-radius:50%;background:${WARNING_HEX};border:1.5px solid white;"></span>`
        : "";
    const blinkClass = state.isBlinking ? "map-node--blink" : "";
    return `<div class="map-node ${blinkClass}" style="position:relative;width:${size}px;height:${size}px;border-radius:50%;background:${color};border:2px solid white;box-shadow:0 0 0 2px ${color}55, 0 1px 3px rgba(0,0,0,.5);">${warning}</div>`;
}

function iconFor(state: NodeState, size: number): L.DivIcon {
    const key = `${state.tone}|${state.hasWarning}|${state.isBlinking}|${size}`;
    let icon = cache.get(key);
    if (!icon) {
        icon = L.divIcon({ className: "", html: nodeHtml(state, size), iconSize: [size, size], iconAnchor: [size / 2, size / 2] });
        cache.set(key, icon);
    }
    return icon;
}

export const nodeIcon = (state: NodeState) => iconFor(state, 18);
export const miniNodeIcon = (state: NodeState) => iconFor({ ...state, hasWarning: false, isBlinking: false }, 9);

export function cityLabelIcon(name: string): L.DivIcon {
    return L.divIcon({ className: "map-city-label", html: name, iconSize: [0, 0] });
}
