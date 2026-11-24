import type { AlertLevel } from "@/services/alertManagement";
import { cn } from "@/lib/utils";
import { levelStyle } from "../../config/alertManagementDisplay";

/** Nivel con su punto de color, sin píldora: el color nunca va solo, siempre con el nombre. */
export function LevelTag({ level }: { level: AlertLevel }) {
    const style = levelStyle(level);
    return (
        <span className="inline-flex items-center gap-1.5 whitespace-nowrap">
            <span aria-hidden className={cn("size-2 rounded-full", style.dotClass)} />
            {style.label}
        </span>
    );
}
