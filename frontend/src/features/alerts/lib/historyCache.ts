import type { AlertItem, AlertSearchResponse } from "@/services/alertManagement";

/**
 * Página del historial con la fila de la alerta reemplazada por la versión que devolvió una escritura.
 * Devuelve la misma referencia si la alerta no está en la página, para no re-renderizar en vano.
 */
export function replaceHistoryItem(
    page: AlertSearchResponse | undefined,
    updated: AlertItem
): AlertSearchResponse | undefined {
    if (!page) return page;
    const index = page.items.findIndex((item) => item.id === updated.id);
    if (index === -1) return page;

    const items = [...page.items];
    // El dispositivo de una alerta no cambia. Se conserva el de la fila porque las respuestas de escritura
    // del backend traen el nombre sin resolver ("MP-<id>") en lugar del nombre del dispositivo.
    items[index] = { ...updated, device: page.items[index].device };
    return { ...page, items };
}

/** Número de páginas de una búsqueda; al menos 1 para que la paginación tenga un estado estable. */
export function pageCount(total: number, pageSize: number): number {
    if (pageSize <= 0) return 1;
    return Math.max(1, Math.ceil(total / pageSize));
}
