import { ChevronLeft, ChevronRight } from "lucide-react";
import { Button } from "@/components/ui/button";
import { paginationRange } from "@/shared/lib/paginationRange";

interface PaginationProps {
    page: number;
    totalPages: number;
    onPageChange: (page: number) => void;
    disabled?: boolean;
    className?: string;
}

/** Anterior / números de página / siguiente. */
export function Pagination({ page, totalPages, onPageChange, disabled = false, className }: PaginationProps) {
    return (
        <nav aria-label="Paginación" className={className}>
            <ul className="flex items-center gap-1">
                <li>
                    <Button
                        variant="outline"
                        size="icon"
                        aria-label="Página anterior"
                        disabled={disabled || page <= 1}
                        onClick={() => onPageChange(page - 1)}
                    >
                        <ChevronLeft aria-hidden />
                    </Button>
                </li>
                {paginationRange(page, totalPages).map((item) =>
                    typeof item === "number" ? (
                        <li key={item}>
                            <Button
                                variant={item === page ? "default" : "ghost"}
                                size="icon"
                                className="font-data-mono"
                                aria-label={`Página ${item}`}
                                aria-current={item === page ? "page" : undefined}
                                disabled={disabled && item !== page}
                                onClick={() => item !== page && onPageChange(item)}
                            >
                                {item}
                            </Button>
                        </li>
                    ) : (
                        <li key={item} aria-hidden className="px-1 text-gray-400">
                            …
                        </li>
                    )
                )}
                <li>
                    <Button
                        variant="outline"
                        size="icon"
                        aria-label="Página siguiente"
                        disabled={disabled || page >= totalPages}
                        onClick={() => onPageChange(page + 1)}
                    >
                        <ChevronRight aria-hidden />
                    </Button>
                </li>
            </ul>
        </nav>
    );
}
