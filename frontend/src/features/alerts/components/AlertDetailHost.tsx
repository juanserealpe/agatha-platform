import { lazy, Suspense } from "react";
import { Loader2, SearchX } from "lucide-react";
import { parseAlertId } from "@/services/alertManagement";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { EmptyState } from "@/shared/components/EmptyState";
import { usePreloadWhenIdle } from "@/shared/hooks/usePreloadWhenIdle";
import { useAlertDetailParam } from "../hooks/useAlertDetailParam";

// El modal de gestión solo se usa al abrir una alerta: se descarga aparte y se precarga en tiempo ocioso.
const loadManagementDialog = () => import("./management/AlertManagementDialog");
const AlertManagementDialog = lazy(() =>
    loadManagementDialog().then((module) => ({ default: module.AlertManagementDialog }))
);

function PlaceholderDialog({ title, loading, onClose }: { title: string; loading: boolean; onClose: () => void }) {
    return (
        <Dialog open onOpenChange={(isOpen) => !isOpen && onClose()}>
            <DialogContent className="sm:max-w-md">
                <DialogTitle className="sr-only">Gestión de alerta</DialogTitle>
                <EmptyState
                    Icon={loading ? Loader2 : SearchX}
                    title={title}
                    className={loading ? "border-none py-8 [&>svg]:animate-spin" : "border-none py-8"}
                />
            </DialogContent>
        </Dialog>
    );
}

/**
 * Punto de entrada del modal de gestión. Se monta una sola vez en el layout raíz y se abre con
 * ?alerta=<id>&gestion=<modo> desde cualquier página.
 */
export function AlertDetailHost() {
    const { alertId: rawId, mode, close } = useAlertDetailParam();
    usePreloadWhenIdle(loadManagementDialog);

    if (rawId === null) return null;

    const alertId = parseAlertId(rawId);
    if (alertId === null) {
        return <PlaceholderDialog title="No se encontró la alerta" loading={false} onClose={close} />;
    }

    return (
        <Suspense fallback={<PlaceholderDialog title="Cargando alerta…" loading onClose={close} />}>
            {/* La clave reinicia el modal (paso y formulario) al abrir otra alerta o modo. */}
            <AlertManagementDialog key={`${alertId}-${mode}`} alertId={alertId} initialMode={mode} onClose={close} />
        </Suspense>
    );
}
