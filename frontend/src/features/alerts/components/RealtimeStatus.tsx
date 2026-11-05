import { useState, useSyncExternalStore } from "react";
import type { LucideIcon } from "lucide-react";
import { Bell, BellOff, BellRing, RefreshCw, Volume2, VolumeX, Wifi, WifiOff } from "lucide-react";
import type { StreamStatus } from "@/services/realtime";
import { cn } from "@/lib/utils";
import { useAlertStreamStatus } from "../hooks/useAlertStream";
import { isAlertSoundMuted, setAlertSoundMuted, subscribeAlertSoundMuted, unlockAlertSound } from "../lib/alertSound";
import {
    browserNotificationPermission,
    requestBrowserNotificationPermission,
    type BrowserNotificationPermission,
} from "../lib/browserNotification";

interface StatusStyle {
    label: string;
    /** Se muestra al pasar el cursor: qué significa para el operador. */
    description: string;
    Icon: LucideIcon;
    className: string;
    spin?: boolean;
}

const POLLING_NOTE = "Mientras tanto, las alertas se actualizan cada 30 s.";

// Verde, ámbar y gris: la escala amarillo/naranja/rojo queda reservada a la criticidad de las alertas.
const STATUS_STYLES: Record<StreamStatus, StatusStyle> = {
    live: {
        label: "En vivo",
        description: "Conectado en tiempo real: las alertas llegan al instante.",
        Icon: Wifi,
        className: "border-emerald-200 bg-emerald-50 text-emerald-700",
    },
    connecting: {
        label: "Conectando",
        description: `Conectando al tiempo real. ${POLLING_NOTE}`,
        Icon: RefreshCw,
        className: "border-amber-200 bg-amber-50 text-amber-800",
        spin: true,
    },
    idle: {
        label: "Conectando",
        description: `Conectando al tiempo real. ${POLLING_NOTE}`,
        Icon: RefreshCw,
        className: "border-amber-200 bg-amber-50 text-amber-800",
        spin: true,
    },
    reconnecting: {
        label: "Reconectando",
        description: `Se perdió la conexión en tiempo real y se está recuperando. ${POLLING_NOTE}`,
        Icon: RefreshCw,
        className: "border-amber-200 bg-amber-50 text-amber-800",
        spin: true,
    },
    offline: {
        label: "Sin conexión",
        description: `No se pudo conectar al tiempo real; se sigue intentando. ${POLLING_NOTE}`,
        Icon: WifiOff,
        className: "border-gray-300 bg-gray-100 text-gray-700",
    },
};

const ICON_BUTTON_CLASS =
    "flex size-8 items-center justify-center rounded-full border border-gray-200 bg-white text-gray-600 transition-colors hover:bg-gray-50 hover:text-gray-900 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-gray-900";

/** Activar las notificaciones del navegador. Se pide con un clic, nunca al cargar la app. */
function NotificationsControl() {
    const [permission, setPermission] = useState<BrowserNotificationPermission>(browserNotificationPermission);

    if (permission === "unsupported") return null;

    if (permission === "granted") {
        return (
            <span className={cn(ICON_BUTTON_CLASS, "cursor-default hover:bg-white")} title="Notificaciones del navegador activadas">
                <Bell size={15} aria-hidden />
                <span className="sr-only">Notificaciones del navegador activadas</span>
            </span>
        );
    }

    if (permission === "denied") {
        const text = "Notificaciones bloqueadas: actívelas en la configuración del sitio del navegador";
        return (
            <span className={cn(ICON_BUTTON_CLASS, "cursor-default text-gray-400 hover:bg-white")} title={text}>
                <BellOff size={15} aria-hidden />
                <span className="sr-only">{text}</span>
            </span>
        );
    }

    return (
        <button
            type="button"
            onClick={async () => {
                // El mismo clic habilita el sonido, que el navegador también bloquea hasta una interacción.
                unlockAlertSound();
                setPermission(await requestBrowserNotificationPermission());
            }}
            title="Recibir una notificación del sistema cuando llegue una alerta con la pestaña oculta"
            className="flex items-center gap-1.5 rounded-full border border-gray-300 bg-white px-3 py-1.5 text-xs font-semibold text-gray-800 transition-colors hover:bg-gray-50 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-gray-900"
        >
            <BellRing size={14} aria-hidden /> Activar avisos
        </button>
    );
}

function SoundControl() {
    const muted = useSyncExternalStore(subscribeAlertSoundMuted, isAlertSoundMuted);
    const label = muted ? "Activar el sonido de los avisos" : "Silenciar el sonido de los avisos";

    return (
        <button
            type="button"
            onClick={() => {
                unlockAlertSound();
                setAlertSoundMuted(!muted);
            }}
            aria-pressed={muted}
            aria-label={label}
            title={label}
            className={ICON_BUTTON_CLASS}
        >
            {muted ? <VolumeX size={15} aria-hidden /> : <Volume2 size={15} aria-hidden />}
        </button>
    );
}

/** Estado de la conexión en tiempo real, con los controles de notificaciones del navegador y de sonido. */
export function RealtimeStatus() {
    const status = STATUS_STYLES[useAlertStreamStatus()];

    return (
        <div className="flex items-center gap-2">
            <div
                role="status"
                title={status.description}
                className={cn(
                    "flex items-center gap-2 rounded-full border px-3 py-1.5 text-sm font-medium shadow-sm",
                    status.className
                )}
            >
                <status.Icon size={16} className={cn(status.spin && "motion-safe:animate-spin")} aria-hidden />
                <span className="font-data-mono text-xs uppercase">{status.label}</span>
                <span className="sr-only">. {status.description}</span>
            </div>
            <NotificationsControl />
            <SoundControl />
        </div>
    );
}
