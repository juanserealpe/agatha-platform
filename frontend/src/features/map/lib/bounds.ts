import L from "leaflet";
import { ROUTE_COORDINATES } from "../data/pipelineData";

/** Encuadre de todo el tramo del piloto. */
export const FULL_BOUNDS = L.latLngBounds(ROUTE_COORDINATES);
