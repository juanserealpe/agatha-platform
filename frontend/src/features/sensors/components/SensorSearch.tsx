import { Search, X } from "lucide-react";

interface SensorSearchProps {
    value: string;
    onChange: (value: string) => void;
}

export function SensorSearch({ value, onChange }: SensorSearchProps) {
    return (
        <div className="relative">
            <Search size={14} className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-gray-400" aria-hidden />
            <input
                type="search"
                value={value}
                onChange={(event) => onChange(event.target.value)}
                onKeyDown={(event) => event.key === "Escape" && onChange("")}
                placeholder="Nombre o PK (ej. 34+470)"
                aria-label="Buscar sensor por nombre o PK"
                className="h-8 w-full rounded-md border border-gray-200 bg-gray-50 pl-8 pr-8 text-xs text-gray-900 placeholder:text-gray-500 focus:border-gray-400 focus:bg-white focus:outline-none [&::-webkit-search-cancel-button]:hidden"
            />
            {value && (
                <button
                    type="button"
                    onClick={() => onChange("")}
                    aria-label="Limpiar búsqueda"
                    className="absolute right-1.5 top-1/2 -translate-y-1/2 rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-600"
                >
                    <X size={12} aria-hidden />
                </button>
            )}
        </div>
    );
}
