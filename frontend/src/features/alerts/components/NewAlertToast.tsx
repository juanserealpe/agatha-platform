import type { CSSProperties } from "react";
import { ArrowRight, BellRing, CircleCheck, Clock, X } from "lucide-react";
import { RelativeTime } from "@/shared/components/RelativeTime";
import { cn } from "@/lib/utils";
import {
    ENDED_CLASSIFIED_TEXT,
    ENDED_NOTICE_LABEL,
    ENDED_UNCLASSIFIED_TEXT,
    LATE_NOTICE_LABEL,
    levelStyle,
    NOTICE_KIND_LABEL,
} from "../config/alertManagementDisplay";
import { useAlertDetailParam } from "../hooks/useAlertDetailParam";
import { useNewAlerts } from "../hooks/useNewAlerts";
import { MAX_VISIBLE_NOTICES, sortNotices, type Notice } from "../lib/newAlerts";

interface NoticeBannerProps {
    notice: Notice;
    onDismiss: () => void;
    onOpen: () => void;
}

function endedText(notice: Notice): string {
    return notice.classification === null ? ENDED_UNCLASSIFIED_TEXT : ENDED_CLASSIFIED_TEXT;
}

function NoticeBanner({ notice, onDismiss, onOpen }: NoticeBannerProps) {
    const severity = levelStyle(notice.level);
    const isEnded = notice.ended === true;
    const isUrgent = notice.level === "ROJO" && !isEnded;
    // Igual que el titileo del mapa: rojo que nadie ha clasificado.
    const pulses = isUrgent && notice.classification === null;
    const HeaderIcon = isEnded ? CircleCheck : BellRing;

    return (
        <div
            role={isUrgent ? "alert" : "status"}
            aria-live={isUrgent ? "assertive" : "polite"}
            style={{ "--glow-color": severity.hex } as CSSProperties}
            className={cn(
                "rounded-xl border-2 p-3 shadow-xl",
                // Finalizada: tono neutro, ya no reclama atención.
                isEnded ? "border-gray-300 bg-white" : cn(severity.borderClass, severity.surfaceClass),
                pulses && "alert-glow"
            )}
        >
            <div className="flex items-center gap-2">
                <HeaderIcon size={16} className={cn("shrink-0", isEnded ? "text-emerald-600" : "text-gray-700")} aria-hidden />
                <p className="text-sm font-bold text-gray-900">{isEnded ? ENDED_NOTICE_LABEL : NOTICE_KIND_LABEL[notice.code]}</p>
                <span className={cn("rounded-full px-2 py-0.5 text-xs font-semibold", severity.badgeClass)}>
                    {severity.label}
                </span>
                {notice.late && (
                    <span className="flex items-center gap-1 whitespace-nowrap rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-700 ring-1 ring-gray-300">
                        <Clock size={12} aria-hidden /> {LATE_NOTICE_LABEL}
                    </span>
                )}
                <button
                    type="button"
                    onClick={onDismiss}
                    aria-label="Descartar aviso"
                    className="ml-auto rounded p-1 text-gray-500 hover:bg-black/5 hover:text-gray-700"
                >
                    <X size={14} aria-hidden />
                </button>
            </div>

            <p className="mt-1.5 text-sm font-semibold text-gray-900">{notice.title}</p>
            {/* El cuerpo del catálogo empieza con la frase simple; el dato queda al final (y completo en el title). */}
            <p className="mt-0.5 line-clamp-2 text-xs text-gray-600" title={isEnded ? undefined : notice.body}>
                {isEnded ? endedText(notice) : notice.body}
            </p>

            <div className="mt-2 flex items-center gap-2 text-xs text-gray-500">
                <span className="truncate font-data-mono">
                    {notice.alert_code} · <RelativeTime value={notice.occurred_at} />
                </span>
                <button
                    type="button"
                    onClick={onOpen}
                    className="ml-auto flex shrink-0 items-center gap-1 whitespace-nowrap rounded px-1.5 py-1 font-semibold text-gray-900 hover:bg-black/5 focus-visible:outline-2 focus-visible:outline-gray-900"
                >
                    Ver detalle <ArrowRight size={14} aria-hidden />
                </button>
            </div>
        </div>
    );
}

/**
 * Avisos de alerta nueva y de subida de nivel, llegados por el stream. Mientras la alerta sigue en curso no se
 * cierran solos: en monitoreo un aviso no debe desaparecer sin que alguien lo vea. Al volver a la normalidad
 * pasan a un tono neutro y se cierran solos al minuto. Uno por alerta, los más graves primero y como máximo tres.
 */
export function NewAlertToast() {
    const { notices, dismiss } = useNewAlerts();
    const { open } = useAlertDetailParam();

    if (notices.length === 0) return null;

    const sorted = sortNotices(notices);
    const visible = sorted.slice(0, MAX_VISIBLE_NOTICES);
    const hidden = sorted.length - visible.length;

    return (
        <div className="flex w-80 max-w-[calc(100%-1.5rem)] flex-col gap-2">
            {visible.map((notice) => (
                <NoticeBanner
                    key={notice.alert_code}
                    notice={notice}
                    onDismiss={() => dismiss(notice.alert_code)}
                    onOpen={() => {
                        dismiss(notice.alert_code);
                        open(String(notice.alert_id));
                    }}
                />
            ))}
            {hidden > 0 && (
                <p className="self-end rounded-full bg-white px-2.5 py-1 text-xs font-medium text-gray-700 shadow-md">
                    +{hidden} {hidden === 1 ? "aviso más" : "avisos más"}
                </p>
            )}
        </div>
    );
}
