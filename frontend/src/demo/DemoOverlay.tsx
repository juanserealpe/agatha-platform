import { useEffect, useReducer, useRef, useState } from "react";
import {
    ArrowUpCircle,
    Check,
    ChevronDown,
    CirclePlus,
    FlaskConical,
    LogOut,
    Play,
    RotateCcw,
    ShieldCheck,
    Timer,
    Wifi,
    WifiOff,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { alertToClassify, alertToRaise, pointForNewAlert } from "./demoActions";
import type { DemoController } from "./installDemo";
import { runAfter } from "./runAfter";
import { SCENARIO_STEPS } from "./scenario";

// Overlay de la demo: una franja que deja claro que los datos son simulados y un panel plegable para manejar la
// simulación (escenario guiado y acciones sueltas). Todo pasa por el store: la app lo ve llegar por el stream,
// igual que con el backend.

const DELAYED_STEP_SECONDS = 5;

export function DemoOverlay({ controller }: { controller: DemoController }) {
    const { store, scenario, stream } = controller;
    // El store, el escenario y el stream viven fuera de React: se vuelve a dibujar en cada cambio.
    const [, refresh] = useReducer((version: number) => version + 1, 0);
    const [open, setOpen] = useState(true);
    const [secondsLeft, setSecondsLeft] = useState<number | null>(null);

    const cancelDelayedStep = useRef<(() => void) | null>(null);

    useEffect(() => store.subscribe(refresh), [store]);
    useEffect(() => () => cancelDelayedStep.current?.(), []);

    // "Disparar en 5 s": da tiempo de cambiar de pestaña o minimizar para ver la notificación del sistema. El
    // paso lo dispara `runAfter`; la cuenta regresiva es solo visual y puede atrasarse con la ventana oculta.
    useEffect(() => {
        if (secondsLeft === null || secondsLeft <= 1) return;
        const timer = setTimeout(() => setSecondsLeft(secondsLeft - 1), 1000);
        return () => clearTimeout(timer);
    }, [secondsLeft]);

    const startDelayedStep = () => {
        setSecondsLeft(DELAYED_STEP_SECONDS);
        cancelDelayedStep.current = runAfter(DELAYED_STEP_SECONDS * 1000, () => {
            cancelDelayedStep.current = null;
            setSecondsLeft(null);
            scenario.advance();
            refresh();
        });
    };

    const cancelDelayed = () => {
        cancelDelayedStep.current?.();
        cancelDelayedStep.current = null;
        setSecondsLeft(null);
    };

    const position = scenario.position();
    const nextStep = scenario.nextStep();
    const connected = stream.isConnected();
    const pointForNew = pointForNewAlert(store);
    const toRaise = alertToRaise(store);
    const toClassify = alertToClassify(store);
    const countingDown = secondsLeft !== null;

    const run = (action: () => void) => () => {
        action();
        refresh();
    };

    return (
        <>
            <div className="mx-auto mb-3 flex max-w-[1600px] flex-wrap items-center justify-between gap-2 rounded-xl border border-amber-300 bg-amber-50 px-4 py-2 text-amber-900">
                <p className="flex items-center gap-2 text-sm font-semibold">
                    <FlaskConical size={16} aria-hidden />
                    DEMO — datos simulados
                    <span className="hidden font-normal text-amber-800 sm:inline">
                        · nada de lo que ves aquí viene del gasoducto ni llega al backend
                    </span>
                </p>
                <Button variant="outline" size="sm" onClick={() => window.location.assign("/")}>
                    <LogOut aria-hidden />
                    Salir de la demo
                </Button>
            </div>

            <section
                aria-label="Panel de simulación"
                className="fixed bottom-4 left-4 z-40 w-80 max-w-[calc(100vw-2rem)] rounded-xl border border-amber-300 bg-white shadow-lg"
            >
                <button
                    type="button"
                    onClick={() => setOpen(!open)}
                    aria-expanded={open}
                    className="flex w-full items-center justify-between gap-2 rounded-xl bg-amber-50 px-4 py-2 text-left text-sm font-semibold text-amber-900"
                >
                    <span className="flex items-center gap-2">
                        <FlaskConical size={16} aria-hidden />
                        Simulación
                        {!connected && <span className="font-normal text-red-700">· sin conexión</span>}
                        {countingDown && <span className="font-normal">· {secondsLeft} s</span>}
                    </span>
                    <ChevronDown size={16} aria-hidden className={cn("transition-transform", open && "rotate-180")} />
                </button>

                {open && (
                    <div className="flex max-h-[70vh] flex-col gap-4 overflow-y-auto p-4">
                        <div className="flex flex-col gap-2">
                            <h2 className="text-xs font-semibold tracking-wider text-gray-500 uppercase">
                                Escenario guiado · Punto 1 (PK 34+470)
                            </h2>
                            <ol className="flex flex-col gap-1.5">
                                {SCENARIO_STEPS.map((step, index) => {
                                    const done = index < position;
                                    const current = index === position;
                                    return (
                                        <li
                                            key={step.id}
                                            className={cn(
                                                "flex gap-2 rounded-lg px-2 py-1 text-sm",
                                                current && "bg-amber-50",
                                                done && "text-gray-400"
                                            )}
                                        >
                                            <span className="mt-0.5 flex size-4 shrink-0 items-center justify-center">
                                                {done ? <Check size={14} aria-label="Hecho" /> : <span className="text-xs">{index + 1}</span>}
                                            </span>
                                            <span className="flex flex-col">
                                                <span className={cn(current && "font-semibold text-gray-900")}>{step.title}</span>
                                                {current && <span className="text-xs text-gray-600">{step.description}</span>}
                                            </span>
                                        </li>
                                    );
                                })}
                            </ol>
                            {!nextStep && <p className="text-xs text-gray-600">El escenario terminó. Reinicia para verlo otra vez.</p>}
                            <div className="flex flex-wrap gap-2">
                                <Button size="sm" disabled={!nextStep || countingDown} onClick={run(() => scenario.advance())}>
                                    <Play aria-hidden />
                                    Siguiente paso
                                </Button>
                                {countingDown ? (
                                    <Button size="sm" variant="outline" onClick={cancelDelayed}>
                                        <Timer aria-hidden />
                                        Cancelar ({secondsLeft} s)
                                    </Button>
                                ) : (
                                    <Button
                                        size="sm"
                                        variant="outline"
                                        disabled={!nextStep}
                                        onClick={startDelayedStep}
                                        title="Da tiempo de cambiar de pestaña para ver la notificación del navegador"
                                    >
                                        <Timer aria-hidden />
                                        Disparar en {DELAYED_STEP_SECONDS} s
                                    </Button>
                                )}
                            </div>
                        </div>

                        <div className="flex flex-col gap-2">
                            <h2 className="text-xs font-semibold tracking-wider text-gray-500 uppercase">Acciones sueltas</h2>
                            <div className="grid grid-cols-2 gap-2">
                                <Button
                                    size="sm"
                                    variant="outline"
                                    disabled={pointForNew === null}
                                    onClick={run(() => pointForNew !== null && store.openAlert("AMARILLO", pointForNew))}
                                >
                                    <CirclePlus aria-hidden />
                                    Alerta nueva
                                </Button>
                                <Button
                                    size="sm"
                                    variant="outline"
                                    disabled={toRaise === null}
                                    onClick={run(() => toRaise !== null && store.raiseLevel(toRaise))}
                                >
                                    <ArrowUpCircle aria-hidden />
                                    Subir nivel
                                </Button>
                                <Button
                                    size="sm"
                                    variant="outline"
                                    disabled={toClassify === null}
                                    onClick={run(
                                        () =>
                                            toClassify !== null &&
                                            store.classify(toClassify, { classification: "CONFIRMADA", origin: "WEB" })
                                    )}
                                >
                                    <ShieldCheck aria-hidden />
                                    Clasificar
                                </Button>
                                <Button
                                    size="sm"
                                    variant={connected ? "outline" : "destructive"}
                                    onClick={run(() => (connected ? stream.disconnect() : stream.reconnect()))}
                                >
                                    {connected ? <WifiOff aria-hidden /> : <Wifi aria-hidden />}
                                    {connected ? "Desconectar" : "Reconectar"}
                                </Button>
                            </div>
                            <p className="text-xs text-gray-500">
                                Actúan sobre la alerta activa más reciente; la alerta nueva se abre en un punto libre.
                                {!connected &&
                                    " Sin conexión, los cambios llegan al reconectar (el cliente reintenta con su espera normal, hasta 30 s)."}
                            </p>
                        </div>

                        {/* Recargar deja todo como al entrar: el store, la caché y los avisos viven en memoria. */}
                        <Button size="sm" variant="ghost" className="self-start" onClick={() => window.location.reload()}>
                            <RotateCcw aria-hidden />
                            Reiniciar la demo
                        </Button>
                    </div>
                )}
            </section>
        </>
    );
}
