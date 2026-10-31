import type { CSSProperties } from "react";
import { ArrowRight, BellRing, Clock, X } from "lucide-react";
import type { AlertNotificationEvent } from "@/services/realtime";
import { RelativeTime } from "@/shared/components/RelativeTime";
import { cn } from "@/lib/utils";
import { LATE_NOTICE_LABEL, levelStyle, NOTICE_KIND_LABEL } from "../config/alertManagementDisplay";
import { useAlertDetailParam } from "../hooks/useAlertDetailParam";
import { useNewAlerts } from "../hooks/useNewAlerts";
import { MAX_VISIBLE_NOTICES, sortNotices } from "../lib/newAlerts";

interface NoticeBannerProps {
    notice: AlertNotificationEvent;
    onDismiss: () => void;
    onOpen: () => void;
}

function NoticeBanner({ notice, onDismiss, onOpen }: NoticeBannerProps) {
    const severity = levelStyle(notice.level);
    const isUrgent = notice.level === "ROJO";
    // Igual que el titileo del mapa: rojo que nadie ha clasificado.
    const pulses = isUrgent && notice.classification === null;

    return (
        <div
            role={isUrgent ? "alert" : "status"}
            aria-live={isUrgent ? "assertive" : "polite"}
            style={{ "--glow-color": severity.hex } as CSSProperties}
            className={cn("rounded-xl border-2 bg-white p-3 shadow-xl", severity.borderClass, pulses && "alert-glow")}
        >
            <div className="flex items-center gap-2">
                <BellRing size={16} className="shrink-0 text-gray-700" aria-hidden />
                <p className="text-sm font-bold text-gray-900">{NOTICE_KIND_LABEL[notice.code]}</p>
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
                    className="ml-auto rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-700"
                >
                    <X size={14} aria-hidden />
                </button>
            </div>

            <p className="mt-1.5 text-sm font-semibold text-gray-900">{notice.title}</p>
            <p className="mt-0.5 line-clamp-2 text-xs text-gray-600" title={notice.body}>
                {notice.body}
            </p>

            <div className="mt-2 flex items-center gap-2 text-xs text-gray-500">
                <span className="truncate font-data-mono">
                    {notice.alert_code} · <RelativeTime value={notice.occurred_at} />
                </span>
                <button
                    type="button"
                    onClick={onOpen}
                    className="ml-auto flex shrink-0 items-center gap-1 whitespace-nowrap rounded px-1.5 py-1 font-semibold text-gray-900 hover:bg-gray-100 focus-visible:outline-2 focus-visible:outline-gray-900"
                >
                    Ver detalle <ArrowRight size={14} aria-hidden />
                </button>
            </div>
        </div>
    );
}

/**
 * Avisos de alerta nueva y de subida de nivel, llegados por el stream. No se cierran solos: en monitoreo un
 * aviso no debe desaparecer sin que alguien lo vea. Uno por alerta, los más graves primero y como máximo tres.
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
