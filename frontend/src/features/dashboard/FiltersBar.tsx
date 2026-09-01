import { Download } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";
import type { FilterOption, FiltersState } from "@/services/types";

interface FiltersBarProps {
    gasoductos: FilterOption[];
    sensores: FilterOption[];
    criticidades: FilterOption[];
    filters: FiltersState;
    onFilterChange: (key: keyof FiltersState, value: string) => void;
    onExport: () => void;
}

export function FiltersBar({
    gasoductos,
    sensores,
    criticidades,
    filters,
    onFilterChange,
    onExport,
}: FiltersBarProps) {
    return (
        <div className="flex flex-col sm:flex-row flex-wrap gap-4 items-stretch sm:items-center p-3 rounded-lg bg-white border border-border shadow-sm">
            <Select
                value={filters.gasoducto}
                onValueChange={(value) => {
                    if (value !== null) onFilterChange("gasoducto", value);
                }}
            >
                <SelectTrigger className="w-full sm:w-[180px] font-data-mono text-sm">
                    <SelectValue placeholder="Gasoducto: todos" />
                </SelectTrigger>
                <SelectContent>
                    {gasoductos.map((z) => (
                        <SelectItem key={z.value} value={z.value}>
                            {z.label}
                        </SelectItem>
                    ))}
                </SelectContent>
            </Select>

            <Select
                value={filters.sensor}
                onValueChange={(value) => {
                    if (value !== null) onFilterChange("sensor", value);
                }}
            >
                <SelectTrigger className="w-full sm:w-[180px] font-data-mono text-sm">
                    <SelectValue placeholder="Sensor: todos" />
                </SelectTrigger>
                <SelectContent>
                    {sensores.map((s) => (
                        <SelectItem key={s.value} value={s.value}>
                            {s.label}
                        </SelectItem>
                    ))}
                </SelectContent>
            </Select>

            <Select
                value={filters.criticidad}
                onValueChange={(value) => {
                    if (value !== null) onFilterChange("criticidad", value);
                }}
            >
                <SelectTrigger className="w-full sm:w-[180px] font-data-mono text-sm">
                    <SelectValue placeholder="Criticidad: todas" />
                </SelectTrigger>
                <SelectContent>
                    {criticidades.map((c) => (
                        <SelectItem key={c.value} value={c.value}>
                            {c.label}
                        </SelectItem>
                    ))}
                </SelectContent>
            </Select>

            <Button
                variant="outline"
                onClick={onExport}
                className="w-full sm:w-auto sm:ml-auto font-data-mono font-medium gap-2 justify-center"
            >
                <Download size={18} className="shrink-0" />
                <span className="hidden md:inline">Exportar datos filtrados (incluye pendientes)</span>
                <span className="inline md:hidden">Exportar datos</span>
            </Button>
        </div>
    );
}