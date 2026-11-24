import { Suspense, useState } from "react";
import { Accordion, AccordionItem, AccordionTrigger, AccordionContent } from "@/components/ui/accordion";
import { usePreloadWhenIdle } from "@/shared/hooks/usePreloadWhenIdle";
import { lazyWithPreload } from "@/shared/lib/lazyWithPreload";
import { SectionHeader } from "./SectionHeader";
import { SeriesSectionSkeleton } from "./SeriesSkeleton";
import { SECTION_ORDER } from "../config/sectionStyles";

// Las gráficas (Recharts + calendario) son lo más pesado de la app y solo se usan al abrir una sección:
// se cargan aparte para que el mapa y los paneles aparezcan antes, y se precargan en tiempo ocioso. Ya
// precargadas, se pintan sin pasar por el fallback de Suspense (ver lazyWithPreload).
const { Component: SeriesSection, preload: loadSeriesSection } = lazyWithPreload(() =>
    import("../sections/SeriesSection").then((module) => module.SeriesSection)
);

export function TelemetryPanel({ deviceId }: { deviceId: number }) {
    const [openSections, setOpenSections] = useState<string[]>([]);
    usePreloadWhenIdle(loadSeriesSection);

    return (
        <Accordion className="gap-3" value={openSections} onValueChange={(v) => setOpenSections(v as string[])}>
            {SECTION_ORDER.map((key) => (
                <AccordionItem
                    key={key}
                    value={key}
                    className="overflow-hidden rounded-xl border border-slate-200 bg-white shadow-xs"
                >
                    <AccordionTrigger
                        className="items-center rounded-none px-4 py-3 hover:bg-slate-50 hover:no-underline"
                        onPointerEnter={() => void loadSeriesSection()}
                    >
                        <SectionHeader sectionKey={key} />
                    </AccordionTrigger>
                    {/* Fondo blanco: las gráficas son mosaicos grises sin borde y deben destacarse sobre él. */}
                    <AccordionContent className="border-t border-slate-200 bg-white p-4">
                        {openSections.includes(key) && (
                            <Suspense fallback={<SeriesSectionSkeleton />}>
                                <SeriesSection key={deviceId} sectionKey={key} deviceId={deviceId} />
                            </Suspense>
                        )}
                    </AccordionContent>
                </AccordionItem>
            ))}
        </Accordion>
    );
}
