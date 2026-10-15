import { useState } from "react";
import { Accordion, AccordionItem, AccordionTrigger, AccordionContent } from "@/components/ui/accordion";
import { SectionHeader } from "./SectionHeader";
import { SeriesSection } from "../sections/SeriesSection";
import { SECTION_ORDER } from "../config/sectionStyles";

export function MetricsAccordion({ deviceId }: { deviceId: number }) {
    const [openSections, setOpenSections] = useState<string[]>([]);

    return (
        <Accordion className="gap-3" value={openSections} onValueChange={(v) => setOpenSections(v as string[])}>
            {SECTION_ORDER.map((key) => (
                <AccordionItem
                    key={key}
                    value={key}
                    className="overflow-hidden rounded-xl border border-slate-200 bg-white shadow-xs"
                >
                    <AccordionTrigger className="items-center rounded-none px-4 py-3 hover:bg-slate-50 hover:no-underline">
                        <SectionHeader sectionKey={key} />
                    </AccordionTrigger>
                    <AccordionContent className="border-t border-slate-200 bg-slate-50/50 p-4">
                        {openSections.includes(key) && <SeriesSection key={deviceId} sectionKey={key} deviceId={deviceId} />}
                    </AccordionContent>
                </AccordionItem>
            ))}
        </Accordion>
    );
}