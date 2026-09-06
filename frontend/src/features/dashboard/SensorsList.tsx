import { getBatteryVisual, getSignalIcon, getAlertVisual } from "@/theme/severity";
import { MapPin } from "lucide-react";
import type { Sensor } from "@/services/types";


interface SensorsListProps {
    sensores: Sensor[];
    onSensorClick: (sensor: Sensor) => void;
}

export function SensorsList({ sensores, onSensorClick }: SensorsListProps) {
    return (
        <div className="flex flex-col gap-2 h-full">
            <h3 className="text-sm font-bold text-gray-900 px-1">
                Sensores ({sensores.length})
            </h3>
            <div className="flex flex-col gap-2 overflow-y-auto">
                {sensores.map((sensor) => {
                    const battery = getBatteryVisual(sensor.bateria);
                    const SignalIcon = getSignalIcon(sensor.senalCelular);
                    const alertVisual = getAlertVisual(sensor.nivelAlerta);

                    return (
                        <button
                            key={sensor.id}
                            type="button"
                            disabled={!alertVisual.clickable}
                            onClick={() => onSensorClick(sensor)}
                            className={`text-left border rounded-lg p-3 flex items-center gap-3 transition-colors cursor-pointer hover:brightness-95 ${alertVisual.ringClass} ${alertVisual.bgClass}`}

                        >
                            <span className={`w-2.5 h-2.5 rounded-full shrink-0 ${alertVisual.dotClass}`} />

                            <div className="flex-1 min-w-0">
                                <div className="flex items-center gap-1.5">
                                    <MapPin size={12} className="text-gray-400 shrink-0" />
                                    <span className="font-data-mono text-xs text-gray-700 truncate">
                                        {sensor.pk}
                                    </span>
                                </div>
                                <p className="text-sm font-medium text-gray-900 truncate">
                                    {sensor.nombre}
                                </p>
                                <p className="text-xs text-gray-400">
                                    Última comunicación: {new Date(sensor.ultimaComunicacion).toLocaleString()}
                                </p>
                            </div>

                            <div className="flex flex-col items-end gap-1 shrink-0">
                                <div className={`flex items-center gap-1 ${battery.colorClass}`}>
                                    <battery.Icon size={16} />
                                    <span className="font-data-mono text-xs">{sensor.bateria}%</span>
                                </div>
                                <SignalIcon size={14} className="text-gray-400" />
                            </div>
                        </button>
                    );
                })}
            </div>
        </div>
    );
}