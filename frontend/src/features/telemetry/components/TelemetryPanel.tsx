import { Suspense, useState } from "react";
import { ChartLine, X } from "lucide-react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { cn } from "@/lib/utils";
import { usePreloadWhenIdle } from "@/shared/hooks/usePreloadWhenIdle";
import { lazyWithPreload } from "@/shared/lib/lazyWithPreload";
import { SeriesSectionSkeleton } from "./SeriesSkeleton";
import { SECTION_ORDER, SECTION_STYLES, type SectionKey } from "../config/sectionStyles";

// Las gráficas (Recharts + calendario) son lo más pesado de la app: se cargan aparte para que el mapa y los paneles
// aparezcan antes, y se precargan en tiempo ocioso. Ya precargadas, se pintan sin pasar por el fallback de Suspense.
const { Component: SeriesSection, preload: loadSeriesSection } = lazyWithPreload(() =>
    import("../sections/SeriesSection").then((module) => module.SeriesSection)
);

interface TelemetryPanelProps {
    deviceId: number;
    deviceName: string;
}

/**
 * Grupos de variables en una columna a la izquierda y sus gráficas a la derecha. Ninguno abre por defecto: no se
 * consulta nada hasta que el operador elige uno, y luego solo ese (los demás, al abrirlos; quedan en caché).
 */
export function TelemetryPanel({ deviceId, deviceName }: TelemetryPanelProps) {
    const [section, setSection] = useState<SectionKey | null>(null);
    usePreloadWhenIdle(loadSeriesSection);

    return (
        <Tabs
            orientation="vertical"
            value={section}
            onValueChange={(value) => setSection(value as SectionKey)}
            onKeyDown={(event) => {
                // Esc cierra la sección. Los eventos de React cruzan portales: el Esc del calendario (que vive en un
                // portal, fuera de este nodo) solo debe cerrar el calendario.
                if (event.key === "Escape" && section && event.currentTarget.contains(event.target as Node)) setSection(null);
            }}
            className="flex-col gap-0 rounded-xl border border-border bg-white shadow-xs lg:flex-row lg:items-start"
        >
            <div
                className={cn(
                    "flex shrink-0 flex-col gap-3 p-3 lg:sticky lg:top-4 lg:w-80",
                    section && "border-b border-slate-200/80 lg:border-b-0"
                )}
            >
                <div className="px-1.5 pt-1">
                    <h2 className="text-base font-semibold text-slate-900">Telemetría</h2>
                    <p className="truncate text-xs text-slate-500" title={deviceName}>
                        {deviceName}
                    </p>
                </div>
                <TabsList aria-label="Grupo de variables" className="h-auto w-full gap-1 bg-transparent p-0">
                    {SECTION_ORDER.map((key) => {
                        const { icon: Icon, label, subtitle, badgeClass } = SECTION_STYLES[key];
                        return (
                            <TabsTrigger
                                key={key}
                                value={key}
                                onPointerEnter={() => void loadSeriesSection()}
                                // Base UI no avisa al pulsar la pestaña ya abierta: ese clic la cierra.
                                onClick={() => {
                                    if (key === section) setSection(null);
                                }}
                                title={key === section ? "Pulsa de nuevo para cerrar" : undefined}
                                className="group/section h-auto justify-start gap-3 rounded-lg px-2.5 py-2.5 text-left whitespace-normal hover:bg-slate-50 data-active:bg-slate-100 data-active:shadow-none"
                            >
                                <span className={cn("flex size-9 shrink-0 items-center justify-center rounded-lg border", badgeClass)}>
                                    <Icon aria-hidden className="size-[18px]" />
                                </span>
                                <span className="flex min-w-0 flex-1 flex-col">
                                    <span className="text-sm font-bold text-slate-900">{label}</span>
                                    <span className="text-xs font-normal text-slate-500">{subtitle}</span>
                                </span>
                                {/* Solo en la abierta: indica que se puede cerrar. */}
                                <X aria-hidden className="hidden size-4 shrink-0 text-slate-400 group-data-active/section:block" />
                            </TabsTrigger>
                        );
                    })}
                </TabsList>
            </div>

            {section === null && (
                // En móvil la lista ya se explica sola; en escritorio, el espacio de la derecha no queda en blanco.
                <div className="hidden min-h-64 flex-1 flex-col items-center justify-center gap-4 self-stretch border-l border-slate-200/80 p-6 text-center lg:flex">
                    <span className="flex size-16 items-center justify-center rounded-2xl bg-slate-100">
                        <ChartLine className="size-8 text-slate-400" aria-hidden />
                    </span>
                    <div className="flex flex-col gap-1">
                        <p className="text-lg font-semibold text-slate-800">Elige un grupo de variables</p>
                        <p className="text-sm text-slate-500">Sus gráficas aparecerán aquí.</p>
                    </div>
                </div>
            )}
            {SECTION_ORDER.map((key) => (
                <TabsContent key={key} value={key} className="min-w-0 self-stretch p-4 lg:border-l lg:border-slate-200/80">
                    <Suspense fallback={<SeriesSectionSkeleton />}>
                        {/* Una instancia por sensor y grupo: la gráfica anterior no se reutiliza con datos de otro. */}
                        <SeriesSection key={`${deviceId}-${key}`} sectionKey={key} deviceId={deviceId} />
                    </Suspense>
                </TabsContent>
            ))}
        </Tabs>
    );
}
