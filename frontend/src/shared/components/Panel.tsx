import { useId, type ReactNode } from "react";
import { cn } from "@/lib/utils";

interface PanelProps {
    title: ReactNode;
    /** Elemento junto al título, p. ej. un contador. */
    badge?: ReactNode;
    /** Elemento alineado a la derecha del encabezado. */
    action?: ReactNode;
    /** Contenido fijo entre el encabezado y el área con scroll, p. ej. un buscador. */
    toolbar?: ReactNode;
    /** Contenido fijo bajo el área con scroll, p. ej. un botón "Ver todas". */
    footer?: ReactNode;
    className?: string;
    children: ReactNode;
}

/**
 * Contenedor con encabezado fijo y cuerpo con scroll interno.
 * La altura la decide el padre (flex/grid); el panel nunca crece más allá de ella.
 */
export function Panel({ title, badge, action, toolbar, footer, className, children }: PanelProps) {
    const titleId = useId();

    return (
        <section
            aria-labelledby={titleId}
            className={cn("flex min-h-0 flex-col rounded-xl border border-border bg-white shadow-xs", className)}
        >
            <header className="flex items-center gap-2 border-b border-border px-4 py-3">
                <h2 id={titleId} className="font-data-mono text-xs font-bold uppercase tracking-wider text-gray-900">
                    {title}
                </h2>
                {badge}
                {action && <div className="ml-auto">{action}</div>}
            </header>

            {toolbar && <div className="border-b border-border px-3 py-2">{toolbar}</div>}

            <div className="min-h-0 flex-1 overflow-y-auto p-3">{children}</div>

            {footer && <footer className="border-t border-border p-3">{footer}</footer>}
        </section>
    );
}
