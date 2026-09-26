// features/dashboard/MetricsAccordion.tsx
import { useMemo, useState } from "react";
import { Accordion, AccordionItem, AccordionTrigger, AccordionContent } from "@/components/ui/accordion";
import { MetricChart } from "./MetricChart";
import { METRIC_SECTIONS, type MetricSectionKey } from "./metricSections";
import type { MetricMeasurements } from "@/services/dashboard/measurement.types";

interface MetricsAccordionProps {
    measurements: MetricMeasurements[];
}

export function MetricsAccordion({ measurements }: MetricsAccordionProps) {
    const [openSections, setOpenSections] = useState<MetricSectionKey[]>([]);

    // clave del fix: esto solo se recalcula cuando measurements cambia de verdad,
    // así los arrays que le llegan a cada MetricChart mantienen su referencia
    // entre renders y React.memo funciona como debe.
    const sectionsWithGroups = useMemo(() => {
        const byId = new Map<number, MetricMeasurements>();
        for (const m of measurements) byId.set(m.metricId, m);

        return (Object.keys(METRIC_SECTIONS) as MetricSectionKey[]).map((key) => {
            const section = METRIC_SECTIONS[key];
            const groups = section.groups
                .map((group) => ({
                    title: group.title,
                    metrics: group.metricIds
                        .map((id) => byId.get(id))
                        .filter((m): m is MetricMeasurements => !!m),
                }))
                .filter((g) => g.metrics.length > 0);
            return { key, label: section.label, groups };
        });
    }, [measurements]);

    return (
        <Accordion value={openSections} onValueChange={(v) => setOpenSections(v as MetricSectionKey[])}>
            {sectionsWithGroups.map(({ key, label, groups }) => {
                const isOpen = openSections.includes(key);
                return (
                    <AccordionItem key={key} value={key}>
                        <AccordionTrigger>
                            {label} ({groups.length})
                        </AccordionTrigger>
                        <AccordionContent>
                            {isOpen &&
                                (groups.length === 0 ? (
                                    <p className="text-xs text-gray-400 py-4 text-center">
                                        Sin datos disponibles para este rango
                                    </p>
                                ) : (
                                    <div className="flex flex-col gap-4">
                                        {groups.map((group) => (
                                            <MetricChart key={group.title} title={group.title} metrics={group.metrics} />
                                        ))}
                                    </div>
                                ))}
                        </AccordionContent>
                    </AccordionItem>
                );
            })}
        </Accordion>
    );
}