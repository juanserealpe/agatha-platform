import { useDevices } from "@/features/sensors";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { deviceOptionLabel } from "../../lib/deviceLabel";

// Valor del contrato para "todos los dispositivos" (equivale a no enviar `device`).
const ALL_DEVICES = "todos";

interface DeviceFilterSelectProps {
    id: string;
    value: number | null;
    onChange: (device: number | null) => void;
}

/** Filtro de dispositivo con la lista de GET /api/dashboard/devices y "Todos" al inicio. */
export function DeviceFilterSelect({ id, value, onChange }: DeviceFilterSelectProps) {
    const { data: devices = [], isPending, isError } = useDevices();

    const options = [
        { value: ALL_DEVICES, label: "Todos" },
        ...devices.map((device) => ({ value: String(device.id), label: deviceOptionLabel(device.id, devices) })),
    ];
    // Un dispositivo aplicado que ya no está en la lista (o que aún no cargó) se sigue mostrando.
    if (value !== null && !devices.some((device) => device.id === value)) {
        options.push({ value: String(value), label: deviceOptionLabel(value, devices) });
    }

    const hintId = `${id}-hint`;

    return (
        <div className="flex flex-col gap-1.5">
            <Label htmlFor={id} className="text-xs font-medium text-gray-600">
                Dispositivo
            </Label>
            <Select
                items={options}
                value={value === null ? ALL_DEVICES : String(value)}
                onValueChange={(next) => onChange(next === null || next === ALL_DEVICES ? null : Number(next))}
            >
                <SelectTrigger id={id} className="w-full" aria-describedby={isError ? hintId : undefined}>
                    <SelectValue />
                </SelectTrigger>
                <SelectContent>
                    {options.map((option) => (
                        <SelectItem key={option.value} value={option.value}>
                            {option.label}
                        </SelectItem>
                    ))}
                </SelectContent>
            </Select>
            {isPending && <p className="text-xs text-gray-500">Cargando dispositivos…</p>}
            {isError && (
                <p id={hintId} className="text-xs text-gray-500">
                    No se pudo cargar la lista de dispositivos.
                </p>
            )}
        </div>
    );
}
