import { Suspense, useState } from "react";
import { SearchX } from "lucide-react";
import { parseAlertId } from "@/services/alertManagement";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { EmptyState } from "@/shared/components/EmptyState";
import { usePreloadWhenIdle } from "@/shared/hooks/usePreloadWhenIdle";
import { lazyWithPreload } from "@/shared/lib/lazyWithPreload";
import { useAlertDetailParam } from "../hooks/useAlertDetailParam";
import { usePrefetchOpenedAlert } from "../hooks/useAlertGroup";
import { AlertDetailSkeleton, MANAGEMENT_DIALOG_CLASS } from "./management/AlertDetailSkeleton";

// El modal de gestión solo se usa al abrir una alerta: se descarga aparte y se precarga en tiempo ocioso. Ya
// precargado, abre sin pasar por el fallback de Suspense (ver lazyWithPreload).
const { Component: AlertManagementDialog, preload: loadManagementDialog } = lazyWithPreload(() =>
    import("./management/AlertManagementDialog").then((module) => module.AlertManagementDialog)
);

function PlaceholderDialog({ loading, onClose }: { loading: boolean; onClose: () => void }) {
    return (
        <Dialog open onOpenChange={(isOpen) => !isOpen && onClose()}>
            {/* Cargando: el mismo ancho y esqueleto que el modal, para que no salte al llegar su código. */}
            <DialogContent className={loading ? MANAGEMENT_DIALOG_CLASS : "sm:max-w-md"}>
                <DialogTitle className="sr-only">Gestión de alerta</DialogTitle>
                {loading ? (
                    <AlertDetailSkeleton />
                ) : (
                    <EmptyState Icon={SearchX} title="No se encontró la alerta" className="border-none py-8" />
                )}
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
    const alertId = rawId === null ? null : parseAlertId(rawId);
    usePrefetchOpenedAlert(alertId);

    if (rawId === null) return null;
    if (alertId === null) {
        return <PlaceholderDialog loading={false} onClose={close} />;
    }

    return (
        <Suspense fallback={<PlaceholderDialog loading onClose={close} />}>
            <AlertManagementDialog key={dialogKey} alertId={alertId} initialMode={mode} onClose={close} />
        </Suspense>
    );
}
