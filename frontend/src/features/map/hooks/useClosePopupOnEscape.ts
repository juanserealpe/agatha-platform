import { useEffect } from "react";
import { useMap } from "react-leaflet";

// Leaflet solo cierra el popup con Esc si el contenedor del mapa tiene el foco; al hacer clic en un
// punto el foco queda en el punto, así que Esc se atiende a nivel de documento mientras haya un popup abierto.
export function useClosePopupOnEscape() {
    const map = useMap();
    useEffect(() => {
        const onKeyDown = (event: KeyboardEvent) => {
            if (event.key === "Escape") map.closePopup();
        };
        const listen = () => document.addEventListener("keydown", onKeyDown);
        const stopListening = () => document.removeEventListener("keydown", onKeyDown);
        map.on("popupopen", listen);
        map.on("popupclose", stopListening);
        return () => {
            map.off("popupopen", listen);
            map.off("popupclose", stopListening);
            stopListening();
        };
    }, [map]);
}
