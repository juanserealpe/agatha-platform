import type { CSSProperties } from "react";
import { ArrowRight, CircleCheck, X } from "lucide-react";
import { useDevices } from "@/features/sensors";
import { RelativeTime } from "@/shared/components/RelativeTime";
import { cn } from "@/lib/utils";
import {
    ENDED_NOTICE_LABEL,
    EVENT_ICON,
    EVENT_LABEL,
    LATE_NOTICE_LABEL,
    levelStyle,
    NOTICE_KIND_LABEL,
    ROW_ACTION_LABEL,
} from "../config/alertManagementDisplay";
import { useAlertDetailParam } from "../hooks/useAlertDetailParam";
import { useNewAlerts } from "../hooks/useNewAlerts";
import { deviceOptionLabel } from "../lib/deviceLabel";
import { ROW_ACTION_MODE } from "../lib/managementMode";
import {
    MAX_VISIBLE_NOTICES,
    noticeAction,
    noticeLevelChange,
    sortNotices,
    type LevelChange,
    type Notice,
} from "../lib/newAlerts";

/** Nivel con su punto de color: el color nunca va solo, siempre con el nombre. */
function LevelTag({ level }: { level: LevelChange["to"] }) {
    const style = levelStyle(level);
    return (
        <span className="inline-flex items-center gap-1.5 whitespace-nowrap">
            <span aria-hidden className={cn("size-2 rounded-full", style.dotClass)} />
            {style.label}
        </span>
    );
}

function levelChangeText(change: LevelChange, ended: boolean): string {
    const to = levelStyle(change.to).label;
    if (ended) return `nivel máximo ${to}`;
    return change.from ? `de ${levelStyle(change.from).label} a ${to}` : `nivel ${to}`;
}

interface NoticeCardProps {
    notice: Notice;
    sensorLabel: string;
    onDismiss: () => void;
    onOpen: () => void;
}

function NoticeCard({ notice, sensorLabel, onDismiss, onOpen }: NoticeCardProps) {
    const isEnded = notice.ended === true;
    const change = noticeLevelChange(notice);
    const severity = levelStyle(change.to);
    const isUrgent = notice.level === "ROJO" && !isEnded;
    // Igual que el titileo del mapa: rojo que nadie ha clasificado.
    const pulses = isUrgent && notice.classification === null;
    const EventIcon = isEnded ? CircleCheck : EVENT_ICON[notice.event_type];
    const kind = isEnded ? ENDED_NOTICE_LABEL : NOTICE_KIND_LABEL[notice.code];
    const actionLabel = ROW_ACTION_LABEL[noticeAction(notice)];

    return (
        <div
            role={isUrgent ? "alert" : "status"}
            aria-live={isUrgent ? "assertive" : "polite"}
            style={{ "--glow-color": severity.hex } as CSSProperties}
            className={cn(
                // Vidrio al estilo macOS: el filtro aclara y desatura lo que hay detrás (el mapa satelital es oscuro
                // y verde) antes del tinte, para que el color del nivel se vea limpio y no turbio.
                "notice-material group relative flex gap-3 rounded-2xl border p-3 shadow-notification backdrop-blur-xl backdrop-brightness-160 backdrop-saturate-50",
                // En curso, vidrio del color del nivel; finalizada, neutra: ya no reclama atención.
                isEnded ? "border-gray-200 bg-white/88" : cn(severity.hairlineClass, severity.glassClass),
                pulses && "notice-glow"
            )}
        >
            {/* Toda la tarjeta abre la alerta: un único control para teclado y lector de pantalla, encima del
                contenido (que no está posicionado). Solo "Descartar" y la hora (por su tooltip) quedan por encima. */}
            <button
                type="button"
                onClick={onOpen}
                aria-label={`${actionLabel}: ${kind}, ${levelChangeText(change, isEnded)}, ${sensorLabel}, ${notice.alert_code}`}
                className="absolute inset-0 rounded-2xl focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-gray-900"
            />

            <button
                type="button"
                onClick={onDismiss}
                aria-label="Descartar aviso"
                className="absolute -top-2 -left-2 z-20 grid size-5 place-items-center rounded-full border border-black/10 bg-white text-gray-500 opacity-0 shadow-sm transition-opacity group-focus-within:opacity-100 group-hover:opacity-100 hover:text-gray-900 focus-visible:opacity-100 focus-visible:outline-2 focus-visible:outline-gray-900 pointer-coarse:size-7 pointer-coarse:opacity-100"
            >
                <X size={12} strokeWidth={2.5} aria-hidden />
            </button>

            <span
                aria-hidden
                className={cn(
                    "grid size-9 shrink-0 place-items-center rounded-[10px] ring-1 ring-black/5 ring-inset",
                    // Sobre el velo del nivel, el mosaico va en blanco para que el ícono se despegue del fondo.
                    isEnded ? "bg-emerald-50 text-emerald-600" : cn("bg-white/80", severity.iconClass)
                )}
            >
                <EventIcon size={18} strokeWidth={2} />
            </span>

            <div className="min-w-0 flex-1">
                <div className="flex items-baseline gap-2 text-xs text-gray-600">
                    <span className="truncate font-medium">{EVENT_LABEL[notice.event_type]}</span>
                    <span className="ml-auto shrink-0 whitespace-nowrap">
                        {notice.late && `${LATE_NOTICE_LABEL} · `}
                        <RelativeTime value={notice.occurred_at} className="relative z-10" />
                    </span>
                </div>

                <p className="mt-0.5 text-sm leading-snug font-semibold tracking-[-0.01em] text-gray-900">{kind}</p>
                <p className="truncate text-[13px] leading-snug text-gray-700">
                    {sensorLabel} · <span className="font-data-mono text-xs text-gray-600">{notice.alert_code}</span>
                </p>

                <div className="mt-2.5 flex items-center gap-2 text-xs font-medium text-gray-700">
                    {isEnded && <span className="text-gray-600">Máximo</span>}
                    {change.from && (
                        <>
                            <LevelTag level={change.from} />
                            <ArrowRight size={12} className="shrink-0 text-gray-500" aria-hidden />
                        </>
                    )}
                    <LevelTag level={change.to} />
                    <span
                        aria-hidden
                        className="ml-auto shrink-0 rounded-full bg-white/65 px-2.5 py-1 font-semibold text-gray-900 ring-1 ring-black/5 transition-colors group-hover:bg-white/90"
                    >
                        {actionLabel}
                    </span>
                </div>
            </div>
        </div>
    );
}

/**
 * Avisos de alerta nueva y de subida de nivel, llegados por el stream. Mientras la alerta sigue en curso no se
 * cierran solos: en monitoreo un aviso no debe desaparecer sin que alguien lo vea. Al volver a la normalidad
 * pasan a un tono neutro y se cierran solos al minuto. Uno por alerta, los más graves primero y como máximo tres.
 * Sin clasificar, el clic abre directo en la clasificación.
 */
export function NewAlertToast() {
    const { notices, dismiss } = useNewAlerts();
    const { open } = useAlertDetailParam();
    const { data: devices = [] } = useDevices();

    if (notices.length === 0) return null;

    const sorted = sortNotices(notices);
    const visible = sorted.slice(0, MAX_VISIBLE_NOTICES);
    const hidden = sorted.length - visible.length;

    return (
        <div className="flex w-84 max-w-[calc(100%-1.5rem)] flex-col gap-2.5">
            {visible.map((notice) => (
                <NoticeCard
                    key={notice.alert_code}
                    notice={notice}
                    // El id del dispositivo del dashboard es el measuring_point_id de Alert Management.
                    sensorLabel={deviceOptionLabel(notice.measuring_point_id, devices)}
                    onDismiss={() => dismiss(notice.alert_code)}
                    onOpen={() => {
                        dismiss(notice.alert_code);
                        open(String(notice.alert_id), ROW_ACTION_MODE[noticeAction(notice)]);
                    }}
                />
            ))}
            {hidden > 0 && (
                <p className="self-end rounded-full bg-white px-2.5 py-1 text-xs font-medium text-gray-700 shadow-notification">
                    +{hidden} {hidden === 1 ? "aviso más" : "avisos más"}
                </p>
            )}
        </div>
    );
}
