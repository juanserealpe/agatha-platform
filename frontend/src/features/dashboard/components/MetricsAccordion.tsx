// features/dashboard/MetricsAccordion.tsx
import { useMemo, useState } from "react";
import { Accordion, AccordionItem, AccordionTrigger, AccordionContent } from "@/components/ui/accordion";
import { MetricChart } from "../MetricChart";
import { FugaSection } from "../sections/FugaSection";
import { METRIC_SECTIONS, type MetricSectionKey } from "../metricSections";
import type { MetricMeasurements } from "@/services/dashboard/measurement.types";

interface MetricsAccordionProps {
    measurements: MetricMeasurements[];
    deviceId?: number;
    startDate: string;
    endDate: string;
}

export function MetricsAccordion({ measurements, deviceId, startDate, endDate }: MetricsAccordionProps) {
    const [openSections, setOpenSections] = useState<string[]>([]);

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
        <Accordion value={openSections} onValueChange={(v) => setOpenSections(v as string[])}>
            {/* Sección nueva: pide sus propios datos al endpoint /series */}
            <AccordionItem value="fuga">
                <AccordionTrigger>Fuga</AccordionTrigger>
                <AccordionContent>
                    {openSections.includes("fuga") && deviceId !== undefined && (
                        <FugaSection deviceId={deviceId} startDate={startDate} endDate={endDate} />
                    )}
                </AccordionContent>
            </AccordionItem>

            {/* Secciones que todavía usan el flujo anterior */}
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