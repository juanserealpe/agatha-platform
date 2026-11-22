import { AlertCircle, Loader2, SearchX } from "lucide-react";
import type { UseQueryResult } from "@tanstack/react-query";
import type { AlertDetail } from "@/services/alertManagement";
import { isApiError, userErrorMessage } from "@/services/http";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/shared/components/EmptyState";

const NOT_FOUND_STATUS = 404;
const LOAD_ERROR = "No fue posible cargar el detalle de la alerta. Intente nuevamente.";

/** Carga, "no encontrada" o error del detalle de una alerta (al abrir el modal y en cada pestaña). */
export function DetailLoadState({ query }: { query: Pick<UseQueryResult<AlertDetail>, "error" | "isPending" | "isError" | "refetch"> }) {
    const { error, isPending, isError, refetch } = query;
    if (isPending) {
        return <EmptyState Icon={Loader2} title="Cargando alerta…" className="border-none py-10 [&>svg]:animate-spin" />;
    }
    if (isApiError(error) && error.status === NOT_FOUND_STATUS) {
        return <EmptyState Icon={SearchX} title="No se encontró la alerta" className="border-none py-10" />;
    }
    if (!isError) return null;
    return (
        <EmptyState
            Icon={AlertCircle}
            title="No se pudo cargar la alerta"
            description={userErrorMessage(error, LOAD_ERROR)}
            action={
                <Button variant="outline" onClick={() => void refetch()}>
                    Reintentar
                </Button>
            }
            className="border-none py-10"
        />
    );
}
