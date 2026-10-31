import type { PathOptions } from "leaflet";
import { SEVERITY_STYLES } from "@/features/alerts";
import type { PipeMaterial } from "../data/pilotSensors";
import type { NodeTone } from "../lib/nodeState";

export const TILE_LAYERS = {
    satellite: {
        url: "https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}",
        attribution: "Tiles &copy; Esri — Source: Esri, Maxar, Earthstar Geographics",
    },
    places: {
        url: "https://server.arcgisonline.com/ArcGIS/rest/services/Reference/World_Boundaries_and_Places/MapServer/tile/{z}/{y}/{x}",
    },
} as const;

// Tubería en tonos neutros: la escala amarillo/naranja/rojo queda reservada a severidad y el azul al agua.
// Trazo oscuro con borde blanco: no se confunde con los límites municipales (blancos) de la capa de lugares.
// El material se distingue por el patrón (continuo / punteado), no solo por el color.
const PIPE_CORE = "#1e293b";
const PIPE_CASING = "#ffffff";

export const PIPE_CASING_STYLE: PathOptions = { color: PIPE_CASING, weight: 7, opacity: 0.95, lineCap: "round" };
// Tapa el trazo de acero bajo los tramos flexibles para que el punteado se lea limpio.
export const FLEXIBLE_MASK_STYLE: PathOptions = { color: PIPE_CASING, weight: 7, opacity: 1, lineCap: "round" };

export const PIPE_STYLES: Record<PipeMaterial, { label: string; path: PathOptions }> = {
    steel: { label: "Acero", path: { color: PIPE_CORE, weight: 3.5, opacity: 1, lineCap: "round" } },
    flexible: { label: "Flexible", path: { color: PIPE_CORE, weight: 3.5, opacity: 1, dashArray: "2 6", lineCap: "round" } },
};

/** Por debajo de este zoom los PK se muestran solo al pasar el cursor, para que no se solapen. */
export const PK_LABELS_MIN_ZOOM = 13;

export const WATER_STYLES = {
    river: { label: "Río", path: { color: "#38bdf8", weight: 2.5, opacity: 0.75 } satisfies PathOptions },
    stream: { label: "Quebrada", path: { color: "#7dd3fc", weight: 1.2, opacity: 0.6 } satisfies PathOptions },
};

export const NODE_TONE_STYLES: Record<NodeTone, { label: string; hex: string }> = {
    normal: { label: "Normal", hex: "#10b981" },
    AMARILLO: { label: "Alerta amarilla", hex: SEVERITY_STYLES.AMARILLO.hex },
    NARANJA: { label: "Alerta naranja", hex: SEVERITY_STYLES.NARANJA.hex },
    ROJO: { label: "Alerta roja", hex: SEVERITY_STYLES.ROJO.hex },
    unavailable: { label: "Sin comunicación / no operativo", hex: "#9ca3af" },
};

/** Orden en que se listan los estados en la leyenda. */
export const LEGEND_TONES: NodeTone[] = ["normal", "AMARILLO", "NARANJA", "ROJO", "unavailable"];

export const WARNING_HEX = NODE_TONE_STYLES.unavailable.hex;
