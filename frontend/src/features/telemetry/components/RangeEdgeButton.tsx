import { cn } from "@/lib/utils";
import { formatDayMonth } from "@/shared/lib/formatDate";

interface RangeEdgeButtonProps {
    label: string;
    value: Date | undefined;
    active: boolean;
    onClick: () => void;
}

/** Casilla de un extremo del rango: muestra la fecha elegida y marca cuál se está editando. */
export function RangeEdgeButton({ label, value, active, onClick }: RangeEdgeButtonProps) {
    return (
        <button
            type="button"
            onClick={onClick}
            aria-pressed={active}
            className={cn(
                "flex flex-col items-start gap-0.5 rounded-md border px-3 py-2 text-left transition-colors",
                "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
                active ? "border-primary bg-muted ring-1 ring-primary" : "border-input hover:bg-muted/60"
            )}
        >
            <span className="text-xs font-medium text-muted-foreground">{label}</span>
            <span className={cn("font-data-mono text-sm", value ? "text-foreground" : "text-muted-foreground")}>
                {value ? `${formatDayMonth(value)} ${value.getFullYear()}` : "Sin elegir"}
            </span>
        </button>
    );
}
