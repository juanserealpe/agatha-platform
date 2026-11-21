import type { AlertLevel, TimelineMessage } from "@/services/alertManagement";
import { formatColombiaDateTime, toIsoString } from "@/shared/lib/formatDate";
import { cn } from "@/lib/utils";
import { levelStyle, MESSAGE_VARIANT_META } from "../../config/alertManagementDisplay";
import { MESSAGE_KIND_DESCRIPTION, MESSAGE_KIND_LABEL } from "../../config/alertExplanations";
import { messageVariant } from "../../lib/timelineMessage";

interface PhaseMessageProps {
    message: TimelineMessage;
    /** Nivel de la fase que contiene el mensaje: define su fondo, no el tipo de mensaje (D8). */
    phaseLevel: AlertLevel;
    /** Cursor sobre el mensaje: la franja marca su hora (cursor de tiempo, §7.2). */
    onHoverChange?: (hovering: boolean) => void;
}

/** Mensaje de una fase de la línea de tiempo, con el fondo suave del color de su fase (spec multiindicador §7). */
export function PhaseMessage({ message, phaseLevel, onHoverChange }: PhaseMessageProps) {
    const variant = MESSAGE_VARIANT_META[messageVariant(message)];
    const { Icon } = variant;

    // title y body ya vienen redactados por el backend: se muestran como texto, nunca como HTML.
    return (
        <li
            onMouseEnter={() => onHoverChange?.(true)}
            onMouseLeave={() => onHoverChange?.(false)}
            className={cn(
                "flex flex-col gap-0.5 rounded-md px-2.5 py-2 ring-1 ring-transparent transition-colors",
                levelStyle(phaseLevel).phaseMessageClass,
            )}
        >
            <span className="flex flex-wrap items-center gap-x-1 font-data-mono text-xs text-gray-600">
                {Icon && <Icon className="size-3.5 shrink-0" aria-hidden />}
                <time dateTime={toIsoString(message.at)}>{formatColombiaDateTime(message.at)}</time> ·{" "}
                <span title={MESSAGE_KIND_DESCRIPTION[message.kind]}>{MESSAGE_KIND_LABEL[message.kind] ?? message.kind}</span>
                {variant.label && <> · {variant.label}</>}
            </span>
            <span className="text-sm font-medium text-gray-900">{message.title}</span>
            {message.body && <span className="text-sm whitespace-pre-line text-gray-700">{message.body}</span>}
        </li>
    );
}
