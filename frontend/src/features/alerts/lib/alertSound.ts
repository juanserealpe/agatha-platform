import type { AlertLevel } from "@/services/alertManagement";
import type { AlertNotificationEvent } from "@/services/realtime";

// Tono de aviso generado con la Web Audio API (sin archivos de audio). Suena una vez por aviso y en una sola
// pestaña; el silencio se recuerda en este navegador.

const MUTED_STORAGE_KEY = "agatha.alertSound.muted";
/** Cuánto retiene una pestaña el turno de un aviso: las demás lo reciben en el mismo instante y no suenan. */
const LOCK_HOLD_MS = 5_000;
const VOLUME = 0.25;

export interface Beep {
    /** Hz. */
    frequency: number;
    /** Segundos desde el inicio del tono. */
    start: number;
    /** Segundos. */
    duration: number;
}

const beeps = (frequency: number, starts: number[], duration = 0.14): Beep[] =>
    starts.map((start) => ({ frequency, start, duration }));

/** Más notas y más agudas a mayor nivel: Rojo suena con dos ráfagas de tres. */
export function tonePattern(level: AlertLevel): Beep[] {
    switch (level) {
        case "ROJO":
            return beeps(1046, [0, 0.18, 0.36, 0.9, 1.08, 1.26], 0.12);
        case "NARANJA":
            return beeps(880, [0, 0.22, 0.44]);
        case "AMARILLO":
            return beeps(660, [0, 0.25]);
    }
}

/** El valor guardado; cualquier cosa distinta de "true" (o sin acceso al almacenamiento) es sonido activo. */
export function parseMuted(raw: string | null): boolean {
    return raw === "true";
}

function readStoredMuted(): boolean {
    try {
        return parseMuted(localStorage.getItem(MUTED_STORAGE_KEY));
    } catch {
        return false;
    }
}

let muted = readStoredMuted();
const mutedListeners = new Set<() => void>();

function notifyMuted() {
    mutedListeners.forEach((notify) => notify());
}

export function isAlertSoundMuted(): boolean {
    return muted;
}

export function setAlertSoundMuted(next: boolean) {
    muted = next;
    try {
        localStorage.setItem(MUTED_STORAGE_KEY, String(next));
    } catch {
        // Sin almacenamiento (p. ej. modo privado): vale solo para esta pestaña.
    }
    notifyMuted();
}

export function subscribeAlertSoundMuted(listener: () => void) {
    mutedListeners.add(listener);
    return () => mutedListeners.delete(listener);
}

if (typeof window !== "undefined") {
    // Silenciar en una pestaña silencia las demás.
    window.addEventListener("storage", (event) => {
        if (event.key !== MUTED_STORAGE_KEY) return;
        muted = parseMuted(event.newValue);
        notifyMuted();
    });
}

let context: AudioContext | null = null;

/**
 * Habilita el audio. Los navegadores solo lo permiten tras una interacción del usuario, así que se llama desde
 * un clic o una tecla (ver `installAlertSoundUnlock`). El contexto se crea aquí y no antes, para no generar
 * avisos del navegador por intentar sonar sin permiso.
 */
export function unlockAlertSound() {
    if (typeof AudioContext === "undefined") return;
    context ??= new AudioContext();
    if (context.state === "suspended") void context.resume();
}

/** Habilita el audio con la primera interacción del usuario en la página. Devuelve cómo quitar los listeners. */
export function installAlertSoundUnlock(): () => void {
    const events = ["pointerdown", "keydown"] as const;
    const onInteraction = () => {
        unlockAlertSound();
        if (context?.state === "running") events.forEach((type) => window.removeEventListener(type, onInteraction));
    };
    events.forEach((type) => window.addEventListener(type, onInteraction));
    return () => events.forEach((type) => window.removeEventListener(type, onInteraction));
}

function playPattern(audio: AudioContext, pattern: Beep[]) {
    const origin = audio.currentTime;
    for (const beep of pattern) {
        const oscillator = audio.createOscillator();
        const gain = audio.createGain();
        const start = origin + beep.start;
        const end = start + beep.duration;
        oscillator.type = "sine";
        oscillator.frequency.value = beep.frequency;
        // Entrada y salida cortas para que no haga "clic".
        gain.gain.setValueAtTime(0, start);
        gain.gain.linearRampToValueAtTime(VOLUME, start + 0.01);
        gain.gain.setValueAtTime(VOLUME, end - 0.02);
        gain.gain.linearRampToValueAtTime(0, end);
        oscillator.connect(gain).connect(audio.destination);
        oscillator.start(start);
        oscillator.stop(end);
    }
}

/**
 * Suena el aviso, salvo que esté silenciado o que el audio no esté habilitado en esta pestaña. Con varias
 * pestañas abiertas suena solo una: la primera que toma el lock del `message_id`. Solo compiten las que
 * pueden sonar, para que no gane una pestaña sin audio habilitado.
 */
export function playAlertSound(notice: AlertNotificationEvent) {
    const audio = context;
    if (muted || !audio || audio.state !== "running") return;

    const play = () => playPattern(audio, tonePattern(notice.level));
    if (!navigator.locks) {
        play();
        return;
    }
    void navigator.locks.request(`agatha-alert-sound-${notice.message_id}`, { ifAvailable: true }, async (lock) => {
        if (!lock) return;
        play();
        await new Promise((resolve) => setTimeout(resolve, LOCK_HOLD_MS));
    });
}
