import { useCallback, useState } from "react";

/**
 * true mientras un elemento `sticky` está pegado al borde superior de su contenedor con scroll (p. ej. un
 * modal). El ref va en un marcador de 1 px puesto justo encima del elemento (`absolute -top-0.5` dentro de
 * él): solo queda fuera del área visible cuando el elemento está pegado arriba.
 */
export function useIsStuck<T extends HTMLElement>(): [(node: T | null) => void, boolean] {
    const [stuck, setStuck] = useState(false);

    const ref = useCallback((node: T | null) => {
        if (!node) return;
        // IntersectionObserver descuenta el recorte de los contenedores con overflow: basta la ventana como raíz.
        const observer = new IntersectionObserver(([entry]) => setStuck(!entry.isIntersecting));
        observer.observe(node);
        // React 19: lo que devuelve un ref de callback se ejecuta al desmontar el elemento.
        return () => observer.disconnect();
    }, []);

    return [ref, stuck];
}
