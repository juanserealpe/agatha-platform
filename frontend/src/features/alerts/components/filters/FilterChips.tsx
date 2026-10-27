import { useId } from "react";
import { Check } from "lucide-react";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { cn } from "@/lib/utils";

interface FilterChipsProps<T extends string> {
    label: string;
    options: readonly T[];
    getLabel: (option: T) => string;
    /** Punto de color opcional (p. ej. la criticidad); el texto siempre acompaña al color. */
    getDotClass?: (option: T) => string;
    /** Estilo propio de la opción seleccionada (por defecto, fondo oscuro). */
    getSelectedClass?: (option: T) => string;
    value: T[];
    onChange: (value: T[]) => void;
}

// Seleccionado: fondo oscuro + check, no solo un cambio de color.
const CHIP_CLASS =
    "rounded-full px-3 text-xs aria-pressed:border-gray-900 aria-pressed:bg-gray-900 aria-pressed:text-white aria-pressed:hover:bg-gray-800";

/** Multiselección con chips. Dentro de un filtro los valores se combinan con O. */
export function FilterChips<T extends string>({
    label,
    options,
    getLabel,
    getDotClass,
    getSelectedClass,
    value,
    onChange,
}: FilterChipsProps<T>) {
    const labelId = useId();

    return (
        <div className="flex flex-col gap-1.5">
            <span id={labelId} className="text-xs font-medium text-gray-600">
                {label}
            </span>
            <ToggleGroup
                multiple
                value={value}
                // Se devuelve en orden canónico y solo con opciones conocidas.
                onValueChange={(next) => onChange(options.filter((option) => next.includes(option)))}
                aria-labelledby={labelId}
                variant="outline"
                size="sm"
                className="flex-wrap"
            >
                {options.map((option) => (
                    <ToggleGroupItem key={option} value={option} className={cn(CHIP_CLASS, getSelectedClass?.(option))}>
                        <Check className="hidden group-aria-pressed/toggle:block" aria-hidden />
                        {getDotClass && (
                            <span className={cn("size-2 rounded-full group-aria-pressed/toggle:hidden", getDotClass(option))} aria-hidden />
                        )}
                        {getLabel(option)}
                    </ToggleGroupItem>
                ))}
            </ToggleGroup>
        </div>
    );
}
