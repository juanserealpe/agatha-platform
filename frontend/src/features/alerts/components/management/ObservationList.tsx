import type { Observation, ObservationKind } from "@/services/alertManagement";
import { formatColombiaDateTime, toIsoString } from "@/shared/lib/formatDate";
import { ORIGIN_LABEL } from "../../config/alertManagementDisplay";

const KIND_LABEL: Record<ObservationKind, string | null> = {
    OBSERVACION: null,
    CONFLICTO_CLASIFICACION: "Clasificación simultánea",
    CONFLICTO_ETIQUETA: "Etiqueta simultánea",
};

interface ObservationListProps {
    observations: Observation[];
}

/** Observaciones registradas, en solo lectura: se agregan pero nunca se editan ni se borran (RN-08). */
export function ObservationList({ observations }: ObservationListProps) {
    if (observations.length === 0) {
        return <p className="text-sm text-gray-500">Sin observaciones registradas.</p>;
    }

    return (
        <ul className="flex flex-col gap-2">
            {observations.map((observation) => {
                const kind = KIND_LABEL[observation.kind];
                return (
                    <li key={observation.id} className="rounded-lg border border-border bg-gray-50 px-3 py-2">
                        {/* Texto escrito por usuarios: se renderiza como texto, nunca como HTML. */}
                        <p className="text-sm whitespace-pre-line wrap-anywhere text-gray-800">{observation.text}</p>
                        <p className="mt-1 font-data-mono text-xs text-gray-500">
                            {kind && <span className="font-semibold text-gray-600">{kind} · </span>}
                            {ORIGIN_LABEL[observation.origin] ?? observation.origin} ·{" "}
                            <time dateTime={toIsoString(observation.received_at)}>
                                {formatColombiaDateTime(observation.received_at)}
                            </time>
                        </p>
                    </li>
                );
            })}
        </ul>
    );
}
