/* eslint-disable react-hooks/purity */
import { useState } from "react";
import {
    AlertTriangle,
    CheckCircle2,
    XCircle,
    Wrench,
    ClipboardEdit,
    Camera,
    X,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import type { Sensor, MotivoObservacion } from "@/services/types";
import { MOTIVOS_OBSERVACION, TIPOS_ALERTA } from "@/services/types";
import { getBannerVisual } from "@/theme/severity";

const SHOW_MANTENIMIENTO = false;

interface AlertBannerProps {
    selectedSensor: Sensor | null;
    onClose: () => void;
    onConfirmar: (sensor: Sensor, motivo: MotivoObservacion | null, observaciones: string) => void;
    onFalsaAlerta: (sensor: Sensor, motivo: MotivoObservacion | null, observaciones: string) => void;
}

export function AlertBanner({
    selectedSensor,
    onClose,
    onConfirmar,
    onFalsaAlerta,
}: AlertBannerProps) {
    const [motivo, setMotivo] = useState<MotivoObservacion | "otro" | "">("");
    const [observacionesLibres, setObservacionesLibres] = useState("");

    function resetAndClose() {
        setMotivo("");
        setObservacionesLibres("");
        onClose();
    }

    // Estado normal: nada seleccionado
    if (!selectedSensor) {
        return (
            <div className="border border-emerald-200 rounded-xl p-5 bg-white flex items-center gap-4 shadow-sm">
                <div className="w-12 h-12 rounded-full bg-emerald-50 flex items-center justify-center text-emerald-600">
                    <CheckCircle2 size={32} />
                </div>
                <div>
                    <h2 className="text-lg font-bold text-gray-900">Sistema Operativo y Estable</h2>
                    <p className="text-sm text-gray-600">
                        Monitoreo continuo activo — Todos los sensores dentro de los parámetros normales.
                    </p>
                </div>
                <div className="ml-auto flex items-center gap-2 text-xs font-data-mono text-emerald-600 bg-emerald-50 px-3 py-1 rounded-full border border-emerald-100">
                    <span className="w-2 h-2 rounded-full bg-emerald-500 animate-pulse" />
                    EN TIEMPO REAL
                </div>
            </div>
        );
    }

    const esAlerta = selectedSensor.nivelAlerta === "roja" || selectedSensor.nivelAlerta === "amarilla";
    const visual = getBannerVisual(selectedSensor.nivelAlerta);
    const motivoFinal = motivo === "otro" ? null : (motivo || null);

    // Sensor verde seleccionado manualmente: detalle simple, sin flujo de confirmación
    if (!esAlerta) {
        return (
            <div className={`${visual.borderClass} ${visual.bgClass} rounded-xl p-5 shadow-sm relative`}>
                <button type="button" onClick={resetAndClose} className="absolute top-4 right-4 text-gray-400 hover:text-gray-600">
                    <X size={18} />
                </button>
                <div className="flex items-center gap-2 mb-1">
                    <CheckCircle2 className="text-alerta-verde" size={20} />
                    <span className={`font-bold text-sm font-data-mono ${visual.labelColorClass}`}>
                        {selectedSensor.nombre} · {selectedSensor.pk}
                    </span>
                </div>
                <p className="text-sm text-gray-600 ml-7">
                    Última comunicación: {new Date(selectedSensor.ultimaComunicacion).toLocaleString()}
                </p>
            </div>
        );
    }

    return (
        <div className={`${visual.borderClass} ${visual.bgClass} rounded-xl p-5 relative overflow-hidden shadow-md`}>
            <button type="button" onClick={resetAndClose} className="absolute top-4 right-4 text-gray-400 hover:text-gray-600 z-10">
                <X size={18} />
            </button>

            <div className="flex flex-col gap-5">
                <div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
                    <div>
                        <div className={`flex items-center gap-2 font-bold text-lg mb-1 font-data-mono tracking-tight ${visual.labelColorClass}`}>
                            <AlertTriangle className="animate-pulse" size={22} />
                            {visual.labelText} · {selectedSensor.pk} · {selectedSensor.nombre}
                        </div>

                        <div className="text-gray-900 font-medium text-xl mb-3">
                            {selectedSensor.tipoAlerta ? TIPOS_ALERTA[selectedSensor.tipoAlerta] : "Alerta"}
                            {selectedSensor.descripcionEvento && ` — ${selectedSensor.descripcionEvento}`}
                        </div>

                        <div className="flex flex-wrap gap-4 text-xs text-gray-600 font-data-mono bg-white p-2.5 rounded border border-gray-200 shadow-sm">
                            {selectedSensor.confianzaDeteccion !== undefined && (
                                <span>Confianza <strong className="text-gray-900 text-sm ml-1">{selectedSensor.confianzaDeteccion}%</strong></span>
                            )}
                            {selectedSensor.aceleracion && (
                                <>
                                    <span className="text-gray-300">|</span>
                                    <span>Aceleración <strong className="text-alerta-roja text-sm ml-1">{selectedSensor.aceleracion}</strong></span>
                                </>
                            )}
                            <span className="text-gray-300">|</span>
                            <span>Hace <strong className="text-gray-900 text-sm ml-1">
                                {Math.round((Date.now() - new Date(selectedSensor.ultimaComunicacion).getTime()) / 60000)} min
                            </strong></span>
                            {selectedSensor.esDatoReal && (
                                <>
                                    <span className="text-gray-300">|</span>
                                    <span className="text-emerald-600 flex items-center gap-1">
                                        <span className="w-2.5 h-2.5 rounded-full bg-emerald-500 animate-pulse" /> Dato real
                                    </span>
                                </>
                            )}
                        </div>
                    </div>

                    <div className="flex flex-col md:flex-row flex-wrap gap-3 w-full md:w-auto">
                        <Button
                            onClick={() => { onConfirmar(selectedSensor, motivoFinal, observacionesLibres); resetAndClose(); }}
                            className="bg-emerald-500 hover:bg-emerald-600 text-white font-bold px-5 py-3 h-auto"
                        >
                            <CheckCircle2 size={20} /> Confirmar alerta
                        </Button>
                        <Button
                            variant="outline"
                            onClick={() => { onFalsaAlerta(selectedSensor, motivoFinal, observacionesLibres); resetAndClose(); }}
                            className="font-semibold px-5 py-3 h-auto"
                        >
                            <XCircle className="text-alerta-roja" size={20} /> Falsa alerta
                        </Button>
                        {SHOW_MANTENIMIENTO && (
                            <Button variant="outline" className="bg-blue-50 text-blue-700 border-blue-200 hover:bg-blue-100 font-semibold px-5 py-3 h-auto">
                                <Wrench size={20} /> Mantenimiento programado
                            </Button>
                        )}
                    </div>
                </div>

                <div className="bg-white border border-gray-200 rounded-lg p-4 shadow-sm">
                    <h4 className="font-data-mono text-sm text-gray-700 mb-3 flex items-center gap-2 font-semibold">
                        <ClipboardEdit size={18} /> Observaciones del evento
                    </h4>
                    <div className="flex flex-col md:flex-row gap-4">
                        <div className="flex-1 flex flex-col gap-2">
                            <select
                                className="w-full bg-gray-50 border border-gray-300 rounded-lg p-3 text-sm text-gray-700"
                                value={motivo}
                                onChange={(e) => setMotivo(e.target.value as MotivoObservacion | "otro" | "")}
                            >
                                <option value="">Seleccione una observación... (opcional)</option>
                                {Object.entries(MOTIVOS_OBSERVACION).map(([value, label]) => (
                                    <option key={value} value={value}>{label}</option>
                                ))}
                                <option value="otro">Otro...</option>
                            </select>
                            {motivo === "otro" && (
                                <Textarea
                                    value={observacionesLibres}
                                    onChange={(e) => setObservacionesLibres(e.target.value)}
                                    placeholder="Especifique..."
                                    className="h-16 resize-none"
                                />
                            )}
                        </div>
                        <button
                            type="button"
                            className="flex flex-col items-center justify-center gap-2 bg-gray-50 border-2 border-dashed border-gray-300 hover:border-blue-400 hover:bg-blue-50 text-gray-500 hover:text-blue-600 rounded-lg p-4 md:w-48 transition-all h-24"
                        >
                            <Camera size={28} />
                            <span className="text-sm font-medium">Subir fotografías</span>
                        </button>
                    </div>
                </div>
            </div>
        </div>
    );
}