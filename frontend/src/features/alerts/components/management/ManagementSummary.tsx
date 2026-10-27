import { useId } from "react";
import type { AlertDetail } from "@/services/alertManagement";
import { CLASSIFICATION_LABEL, labelDisplayText } from "../../config/alertManagementDisplay";
import { managementMeta } from "../../lib/managementMeta";
import { ManagementStatusBadge } from "../badges/ManagementStatusBadge";
import { StepHeading } from "./StepHeading";

interface ManagementSummaryProps {
    detail: Pick<AlertDetail, "management_status" | "classification_detail" | "label_detail">;
}

function Row({ term, value, meta }: { term: string; value: string; meta?: string }) {
    return (
        <div className="grid grid-cols-[8rem_1fr] gap-2 py-2 not-last:border-b not-last:border-border">
            <dt className="text-xs font-medium text-gray-500">{term}</dt>
            <dd className="text-sm text-gray-900">
                {value}
                {meta && <span className="block font-data-mono text-xs text-gray-500">{meta}</span>}
            </dd>
        </div>
    );
}

/** Clasificación y etiqueta registradas, con su origen y fecha, en solo lectura (CA-2.2.4). */
export function ManagementSummary({ detail }: ManagementSummaryProps) {
    const titleId = useId();
    const { classification_detail: classification, label_detail: label } = detail;

    return (
        <section aria-labelledby={titleId} className="flex flex-col gap-2">
            <StepHeading id={titleId} title="Gestión de la alerta" aside={<ManagementStatusBadge status={detail.management_status} />} />
            <dl className="rounded-lg border border-border px-3">
                <Row
                    term="Clasificación"
                    value={classification ? CLASSIFICATION_LABEL[classification.value] : "Sin clasificar"}
                    meta={classification ? managementMeta(classification) : undefined}
                />
                <Row
                    term="Etiqueta"
                    value={label ? (labelDisplayText(label.value, label.other_text) ?? "") : "Sin etiqueta"}
                    meta={label ? managementMeta(label) : undefined}
                />
            </dl>
        </section>
    );
}
