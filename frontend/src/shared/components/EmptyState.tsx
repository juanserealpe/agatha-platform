import type { LucideIcon } from "lucide-react";
import { cn } from "@/lib/utils";

interface EmptyStateProps {
    title: string;
    description?: string;
    Icon?: LucideIcon;
    className?: string;
}

/** Mensaje para listas vacías, cargas y errores. */
export function EmptyState({ title, description, Icon, className }: EmptyStateProps) {
    return (
        <div
            className={cn(
                "flex flex-col items-center justify-center gap-2 rounded-lg border border-dashed border-gray-300 p-6 text-center",
                className
            )}
        >
            {Icon && <Icon size={24} className="text-gray-400" aria-hidden />}
            <p className="text-sm font-medium text-gray-600">{title}</p>
            {description && <p className="text-xs text-gray-500">{description}</p>}
        </div>
    );
}
