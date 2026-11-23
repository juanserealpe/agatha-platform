import type { CSSProperties, ReactNode } from "react";
import { cn } from "@/lib/utils";

/**
 * Bloque gris de un esqueleto. Su tamaño y forma van en `className` (p. ej. "h-3 w-40", "rounded-full"); `style`
 * solo para medidas que vienen de una constante en px (p. ej. el alto de una gráfica).
 */
export function SkeletonBlock({ className, style }: { className?: string; style?: CSSProperties }) {
    return <div className={cn("rounded bg-gray-200", className)} style={style} />;
}

interface SkeletonProps {
    /** Lo que se anuncia a lectores de pantalla, p. ej. "Cargando alertas…". */
    label: string;
    children: ReactNode;
    /** Clases del contenedor de los bloques (disposición, alto mínimo). */
    className?: string;
}

/**
 * Esqueleto de carga: imita la forma de lo que va a aparecer, así la vista no salta al llegar los datos.
 * - Un solo pulso de opacity en el contenedor, no uno por bloque: una capa compuesta en GPU, sin repintar en cada
 *   frame. Con movimiento reducido queda estático.
 * - Se anuncia una vez con `role="status"`; los bloques quedan ocultos para lectores de pantalla.
 * - min-w-0: dentro de una grilla o un flex, sin esto el ancho mínimo sale de los anchos fijos de los bloques y en un
 *   celular el esqueleto se desborda en vez de encoger los bloques.
 */
export function Skeleton({ label, children, className }: SkeletonProps) {
    return (
        <div role="status" className="min-w-0">
            <span className="sr-only">{label}</span>
            <div aria-hidden className={cn("motion-safe:animate-pulse", className)}>
                {children}
            </div>
        </div>
    );
}
