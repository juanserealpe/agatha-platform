import { useState } from "react";
import { AlertTriangle, CheckCircle2, MapPin } from "lucide-react";
import {
    Dialog,
    DialogContent,
    DialogHeader,
    DialogTitle,
    DialogFooter,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import type { Sensor, MotivoObservacion } from "@/services/types";
import { MOTIVOS_OBSERVACION } from "@/services/types";

// Oculto por ahora — la asesora pidió dejarlo listo por si se reactiva más adelante
const SHOW_MANTENIMIENTO = false;

interface AlertModalProps {
    sensor: Sensor | null;
    open: boolean;
    onOpenChange: (open: boolean) => void;
    onConfirmar: (sensor: Sensor, motivo: MotivoObservacion | null, observaciones: string) => void;
    onFalsaAlerta: (sensor: Sensor, motivo: MotivoObservacion | null, observaciones: string) => void;
}

export function AlertModal({
    sensor,
    open,
    onOpenChange,
    onConfirmar,
    onFalsaAlerta,
}: AlertModalProps) {
    const [motivo, setMotivo] = useState<MotivoObservacion | null>(null);
    const [observaciones, setObservaciones] = useState("");

    if (!sensor) return null;

    const esAlerta = sensor.nivelAlerta === "roja" || sensor.nivelAlerta === "amarilla";

    function resetAndClose() {
        setMotivo(null);
        setObservaciones("");
        onOpenChange(false);
    }

    function handleConfirmar() {
        onConfirmar(sensor!, motivo, observaciones);
        resetAndClose();
    }

    function handleFalsaAlerta() {
        onFalsaAlerta(sensor!, motivo, observaciones);
        resetAndClose();
    }

    return (
        <Dialog open={open} onOpenChange={(o) => (o ? onOpenChange(o) : resetAndClose())}>
            <DialogContent className="sm:max-w-md">
                <DialogHeader>
                    <DialogTitle className="flex items-center gap-2">
                        {esAlerta ? (
                            <AlertTriangle className="text-alerta-roja" size={20} />
                        ) : (
                            <CheckCircle2 className="text-alerta-verde" size={20} />
                        )}
                        {sensor.nombre}
                    </DialogTitle>
                </DialogHeader>

                <div className="flex items-center gap-1.5 text-sm text-gray-500 font-data-mono">
                    <MapPin size={14} />
                    {sensor.pk}
                </div>

                {!esAlerta ? (
                    // Sensor en estado normal: solo detalle de lectura, sin flujo de confirmación
                    <div className="text-sm text-gray-600 space-y-1 py-2">
                        <p>Estado: Normal</p>
                        <p>Última comunicación: {new Date(sensor.ultimaComunicacion).toLocaleString()}</p>
                        <p>Batería: {sensor.bateria}%</p>
                    </div>
                ) : (
                    <div className="space-y-4 py-2">
                        <div>
                            <label className="text-sm font-medium text-gray-700 block mb-1.5">
                                Motivo (opcional)
                            </label>
                            <select
                                className="w-full border border-input rounded-md px-3 py-2 text-sm bg-transparent"
                                value={motivo ?? ""}
                                onChange={(e) =>
                                    setMotivo((e.target.value || null) as MotivoObservacion | null)
                                }
                            >
                                <option value="">Sin especificar</option>
                                {Object.entries(MOTIVOS_OBSERVACION).map(([value, label]) => (
                                    <option key={value} value={value}>
                                        {label}
                                    </option>
                                ))}
                            </select>
                        </div>

                        <div>
                            <label className="text-sm font-medium text-gray-700 block mb-1.5">
                                Observaciones (opcional)
                            </label>
                            <Textarea
                                value={observaciones}
                                onChange={(e) => setObservaciones(e.target.value)}
                                placeholder="Detalles adicionales..."
                            />
                        </div>
                    </div>
                )}

                {esAlerta && (
                    <DialogFooter className="flex-col sm:flex-row gap-2">
                        {SHOW_MANTENIMIENTO && (
                            <Button variant="outline" className="sm:mr-auto">
                                Mantenimiento
                            </Button>
                        )}
                        <Button variant="outline" onClick={handleFalsaAlerta}>
                            Falsa alerta
                        </Button>
                        <Button onClick={handleConfirmar} className="bg-alerta-roja hover:bg-red-600">
                            Confirmar
                        </Button>
                    </DialogFooter>
                )}
            </DialogContent>
        </Dialog>
    );
}