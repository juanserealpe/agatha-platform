// Fábrica de query keys: invalidar alertKeys.all refresca todas las consultas de alertas.
export const alertKeys = {
    all: ["alerts"] as const,
    recent: (limit: number) => [...alertKeys.all, "recent", limit] as const,
    pending: () => [...alertKeys.all, "pending"] as const,
    detail: (id: string) => [...alertKeys.all, "detail", id] as const,
};
