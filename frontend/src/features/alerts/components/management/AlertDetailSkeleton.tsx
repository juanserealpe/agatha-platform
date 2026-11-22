import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

/** Clases del modal de gestión: las comparten el modal y su esqueleto, para que el ancho no salte al cargar. */
export const MANAGEMENT_DIALOG_CLASS = "max-h-[90vh] gap-5 overflow-y-auto p-5 sm:max-w-2xl";

const LOADING_TEXT = "Cargando alerta…";

// max-h-[90vh] del modal menos su relleno p-5 (arriba y abajo).
const SKELETON_FULL_HEIGHT_CLASS = "min-h-[calc(90vh-2.5rem)]";

// Un solo pulso de opacity en el contenedor (no uno por bloque): una capa compuesta, sin repintar en cada frame.
// Con movimiento reducido queda estático.
const PULSE_CLASS = "motion-safe:animate-pulse";

function Bone({ className }: { className?: string }) {
    return <div className={cn("rounded bg-gray-200", className)} />;
}

function SectionHeading({ className }: { className?: string }) {
    return <Bone className={cn("h-3", className)} />;
}

function HeaderSkeleton() {
    return (
        <div className="flex flex-col gap-2 pr-8">
            <div className="flex items-center gap-2">
                <Bone className="h-5 w-64" />
                <Bone className="h-5 w-20 rounded-full" />
            </div>
            <Bone className="h-3 w-80 max-w-full" />
        </div>
    );
}

/** Franja de tiempo: eje arriba y una fila por alerta (columna de nombre y barra). */
function StripRows({ rows }: { rows: number }) {
    return (
        <div className="flex flex-col gap-4 rounded-lg border border-border p-3">
            <div className="flex justify-end gap-16 pr-6">
                <Bone className="h-3 w-12" />
                <Bone className="h-3 w-12" />
                <Bone className="h-3 w-12" />
            </div>
            {Array.from({ length: rows }, (_, index) => (
                <div key={index} className="flex items-center gap-4">
                    <div className="flex w-48 shrink-0 flex-col gap-1.5">
                        <Bone className="h-4 w-32" />
                        <Bone className="h-3 w-40" />
                    </div>
                    <Bone className="h-2 flex-1 rounded-full" />
                </div>
            ))}
        </div>
    );
}

function ContentBones() {
    return (
        <>
            <div className="flex flex-col gap-2">
                <SectionHeading className="w-48" />
                <div className="flex flex-col gap-2 rounded-lg border border-border p-3">
                    <Bone className="h-3 w-full" />
                    <Bone className="h-3 w-11/12" />
                    <Bone className="h-3 w-4/5" />
                    <Bone className="h-3 w-3/5" />
                </div>
            </div>
            <div className="flex flex-col gap-2">
                <div className="flex justify-between">
                    <SectionHeading className="w-44" />
                    <SectionHeading className="w-28" />
                </div>
                <div className="flex flex-col gap-5 rounded-lg border border-border p-3">
                    {[0, 1].map((row) => (
                        <div key={row} className="flex gap-3">
                            <Bone className="size-3 shrink-0 rounded-full" />
                            <div className="flex flex-1 flex-col gap-1.5">
                                <Bone className="h-4 w-40" />
                                <Bone className="h-3 w-56" />
                            </div>
                        </div>
                    ))}
                </div>
            </div>
            <div className="flex flex-col gap-2">
                <div className="flex justify-between">
                    <SectionHeading className="w-36" />
                    <Bone className="h-5 w-24 rounded-full" />
                </div>
                <Bone className="h-10 w-full rounded-lg" />
            </div>
            <div className="-mx-5 -mb-5 mt-auto flex justify-end gap-2 border-t border-border bg-muted/50 px-5 py-4">
                <Bone className="h-8 w-20 rounded-lg" />
                <Bone className="h-8 w-24 rounded-lg" />
            </div>
        </>
    );
}

function Loading({ children, className }: { children: ReactNode; className?: string }) {
    return (
        // min-w-0: en la grilla del modal, sin esto el ancho mínimo sale de los anchos fijos de los bloques y en un
        // celular el esqueleto se desborda en vez de encoger los bloques.
        <div role="status" className="min-w-0">
            <span className="sr-only">{LOADING_TEXT}</span>
            <div aria-hidden className={cn("flex flex-col gap-5", PULSE_CLASS, className)}>
                {children}
            </div>
        </div>
    );
}

/**
 * Esqueleto del modal completo (cabecera, franja y contenido) mientras llegan el detalle y el grupo. Tiene la forma
 * de una alerta sola, el caso más común, y el alto máximo del modal: el detalle casi siempre lo llena, así que al
 * llegar el modal no cambia de tamaño. No incluye el DialogTitle: lo pone quien lo usa.
 */
export function AlertDetailSkeleton() {
    return (
        <Loading className={SKELETON_FULL_HEIGHT_CLASS}>
            <HeaderSkeleton />
            <StripRows rows={1} />
            <ContentBones />
        </Loading>
    );
}

/** Contenido de una pestaña cuyo detalle todavía no llega (la cabecera y la franja ya están). */
export function AlertContentSkeleton() {
    return (
        <Loading>
            <ContentBones />
        </Loading>
    );
}

/** Lugar de la franja si el grupo tarda más de lo esperado y el detalle ya se muestra. */
export function TimeStripSkeleton() {
    return (
        <Loading>
            <StripRows rows={1} />
        </Loading>
    );
}
