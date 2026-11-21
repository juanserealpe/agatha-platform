import { useCallback, useState } from "react";

/**
 * Ancho actual (px) de un elemento, actualizado al cambiar de tamaño. Devuelve un ref de callback para
 * ponerlo en el elemento; el ancho es 0 hasta la primera medición.
 */
export function useElementWidth<T extends HTMLElement>(): [(node: T | null) => void, number] {
    const [width, setWidth] = useState(0);

    const ref = useCallback((node: T | null) => {
        if (!node) return;
        setWidth(node.getBoundingClientRect().width);
        const observer = new ResizeObserver(([entry]) => setWidth(entry.contentRect.width));
        observer.observe(node);
        // React 19: lo que devuelve un ref de callback se ejecuta al desmontar el elemento.
        return () => observer.disconnect();
    }, []);

    return [ref, width];
}
