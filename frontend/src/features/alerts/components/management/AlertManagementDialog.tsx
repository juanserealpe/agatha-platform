import { AlertCircle, Loader2, SearchX } from "lucide-react";
import type { AlertId } from "@/services/alertManagement";
import { isApiError, userErrorMessage } from "@/services/http";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { EmptyState } from "@/shared/components/EmptyState";
import { useAlertManagementDetail } from "../../hooks/useAlertManagementDetail";
import type { ManagementMode } from "../../lib/managementMode";
import { ManagementContent } from "./ManagementContent";

const NOT_FOUND_STATUS = 404;
const LOAD_ERROR = "No fue posible cargar el detalle de la alerta. Intente nuevamente.";

const MANAGEMENT_DIALOG_CLASS = "max-h-[90vh] gap-5 overflow-y-auto p-5 sm:max-w-2xl";

interface AlertManagementDialogProps {
    /** ID de la ALERTA (items[].id), nunca el del dispositivo. */
    alertId: AlertId;
    initialMode: ManagementMode;
    onClose: () => void;
}

/**
 * Modal reutilizable de gestión de una alerta (historial y, más adelante, Centro de Control).
 * El detalle se consulta solo mientras el modal está abierto. Foco atrapado y cierre con Escape los da Dialog.
 */
export function AlertManagementDialog({ alertId, initialMode, onClose }: AlertManagementDialogProps) {
    const { data: detail, error, isPending, isError, refetch } = useAlertManagementDetail(alertId);
    const notFound = isApiError(error) && error.status === NOT_FOUND_STATUS;

    return (
        <Dialog open onOpenChange={(isOpen) => !isOpen && onClose()}>
            <DialogContent className={MANAGEMENT_DIALOG_CLASS}>
                {detail ? (
                    <ManagementContent detail={detail} initialMode={initialMode} onClose={onClose} />
                ) : (
                    <>
                        <DialogTitle className="sr-only">Gestión de alerta</DialogTitle>
                        {isPending ? (
                            <EmptyState Icon={Loader2} title="Cargando alerta…" className="border-none py-10 [&>svg]:animate-spin" />
                        ) : notFound ? (
                            <EmptyState Icon={SearchX} title="No se encontró la alerta" className="border-none py-10" />
                        ) : isError ? (
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
                        ) : null}
                    </>
                )}
            </DialogContent>
        </Dialog>
    );
}
