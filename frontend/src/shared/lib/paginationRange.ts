export type PageItem = number | "ellipsis-start" | "ellipsis-end";

// Primera, última, actual y una a cada lado, más dos posibles "…": con hasta 7 páginas se muestran todas.
const MAX_PAGES_WITHOUT_ELLIPSIS = 7;
const SIBLINGS = 1;

/** Páginas a mostrar en la paginación, p. ej. [1, "ellipsis-start", 4, 5, 6, "ellipsis-end", 12]. */
export function paginationRange(current: number, total: number): PageItem[] {
    if (total <= MAX_PAGES_WITHOUT_ELLIPSIS) {
        return Array.from({ length: Math.max(total, 1) }, (_, index) => index + 1);
    }

    const page = Math.min(Math.max(current, 1), total);
    // Cerca de un extremo se muestran más páginas de ese lado para que el ancho no salte al avanzar.
    const edgeBlock = 3 + 2 * SIBLINGS;
    if (page <= edgeBlock - SIBLINGS) {
        return [...Array.from({ length: edgeBlock }, (_, index) => index + 1), "ellipsis-end", total];
    }
    if (page >= total - edgeBlock + 1 + SIBLINGS) {
        return [1, "ellipsis-start", ...Array.from({ length: edgeBlock }, (_, index) => total - edgeBlock + 1 + index)];
    }
    return [1, "ellipsis-start", page - SIBLINGS, page, page + SIBLINGS, "ellipsis-end", total];
}
