import { useEffect } from "react";
import { useMap } from "react-leaflet";
import { FULL_BOUNDS } from "../lib/bounds";

/** Encuadra todo el tramo al montar el mapa, con un margen en píxeles. */
export function FitBoundsOnMount({ padding = 40 }: { padding?: number }) {
    const map = useMap();
    useEffect(() => {
        map.fitBounds(FULL_BOUNDS, { padding: [padding, padding] });
    }, [map, padding]);
    return null;
}

// Leaflet solo escucha el resize de la ventana; si el contenedor cambia por el layout
// (grid, paneles, fuentes que cargan tarde) hay que avisarle o deja zonas grises sin tiles.
export function InvalidateSizeOnResize() {
    const map = useMap();
    useEffect(() => {
        const observer = new ResizeObserver(() => map.invalidateSize());
        observer.observe(map.getContainer());
        return () => observer.disconnect();
    }, [map]);
    return null;
}
