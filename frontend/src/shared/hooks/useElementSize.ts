import { useCallback, useState } from "react";

export interface ElementSize {
    width: number;
    height: number;
}

const EMPTY_SIZE: ElementSize = { width: 0, height: 0 };

/**
 * Tamaño actual (px, caja de borde, sin transformaciones) de un elemento, actualizado al cambiar de tamaño. Devuelve un ref de
 * callback para ponerlo en el elemento; el tamaño es 0 hasta la primera medición.
 */
export function useElementSize<T extends HTMLElement>(): [(node: T | null) => void, ElementSize] {
    const [size, setSize] = useState<ElementSize>(EMPTY_SIZE);

    const ref = useCallback((node: T | null) => {
        if (!node) return;
        const measure = () => {
            // offset*: tamaño de layout, sin la escala de la animación de apertura del modal.
            const width = node.offsetWidth;
            const height = node.offsetHeight;
            setSize((current) => (current.width === width && current.height === height ? current : { width, height }));
        };
        measure();
        const observer = new ResizeObserver(measure);
        observer.observe(node);
        // React 19: lo que devuelve un ref de callback se ejecuta al desmontar el elemento.
        return () => observer.disconnect();
    }, []);

    return [ref, size];
}
