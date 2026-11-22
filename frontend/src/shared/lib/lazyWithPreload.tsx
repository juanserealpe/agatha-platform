import { lazy, useState, type ComponentType } from "react";

/**
 * Como lazy(), pero si el módulo ya se descargó (por la precarga) el componente se pinta directo, sin Suspense.
 * Por qué: lazy() suspende siempre en su primer render aunque el import() ya haya resuelto, y React 19 mantiene el
 * fallback de Suspense al menos ~300 ms una vez mostrado; eso sumaba 300 ms a la primera apertura.
 * `preload` es estable: se puede pasar a usePreloadWhenIdle.
 */
export function lazyWithPreload<P extends object>(load: () => Promise<ComponentType<P>>) {
    let loaded: ComponentType<P> | null = null;
    const preload = () =>
        load().then((component) => {
            loaded = component;
            return component;
        });
    const LazyComponent = lazy(() => preload().then((component) => ({ default: component })));

    function PreloadedComponent(props: P) {
        // Se elige al montar: cambiar de tipo después lo volvería a montar y perdería su estado.
        const [Component] = useState<ComponentType<P>>(() => loaded ?? LazyComponent);
        return <Component {...props} />;
    }
    return { Component: PreloadedComponent, preload };
}
