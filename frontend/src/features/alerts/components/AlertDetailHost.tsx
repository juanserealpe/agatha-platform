import { lazy, Suspense, useState } from "react";
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
 * Clave del modal: cambia (y lo reinicia: paso y formulario) al abrir otra alerta u otro modo, pero no al cambiar de
 * pestaña en la vista combinada, donde cada pestaña conserva su estado mientras el modal siga abierto.
 */
function useDialogKey(rawId: string | null, mode: string, tabSwitch: boolean): string {
    const [session, setSession] = useState({ rawId, mode, count: 0 });
    // Patrón de React para derivar estado de props: se ajusta en el mismo render, sin efecto.
    if (session.rawId !== rawId || session.mode !== mode) {
        setSession({ rawId, mode, count: tabSwitch ? session.count : session.count + 1 });
    }
    return String(session.count);
}

/**
 * Punto de entrada del modal de gestión. Se monta una sola vez en el layout raíz y se abre con
 * ?alerta=<id>&gestion=<modo> desde cualquier página.
 */
export function AlertDetailHost() {
    const { alertId: rawId, mode, close, tabSwitch } = useAlertDetailParam();
    const dialogKey = useDialogKey(rawId, mode, tabSwitch);
    usePreloadWhenIdle(loadManagementDialog);

    if (rawId === null) return null;

    const alertId = parseAlertId(rawId);
    if (alertId === null) {
        return <PlaceholderDialog title="No se encontró la alerta" loading={false} onClose={close} />;
    }

    return (
        <Suspense fallback={<PlaceholderDialog title="Cargando alerta…" loading onClose={close} />}>
            <AlertManagementDialog key={dialogKey} alertId={alertId} initialMode={mode} onClose={close} />
        </Suspense>
    );
}
