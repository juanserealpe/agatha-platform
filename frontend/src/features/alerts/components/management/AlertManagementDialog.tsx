import { useRef, useState } from "react";
import type { AlertId } from "@/services/alertManagement";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { useAlertManagementDetail } from "../../hooks/useAlertManagementDetail";
import type { ManagementMode } from "../../lib/managementMode";
import { AlertDetailSkeleton, MANAGEMENT_DIALOG_CLASS } from "./AlertDetailSkeleton";
import { AlertGroupView } from "./AlertGroupView";
import { DetailLoadState } from "./DetailLoadState";

interface AlertManagementDialogProps {
    /** ID de la ALERTA (items[].id), nunca el del dispositivo. En la vista combinada, la pestaña abierta. */
    alertId: AlertId;
    initialMode: ManagementMode;
    onClose: () => void;
}

/**
 * Modal reutilizable de gestión de una alerta (historial y, más adelante, Centro de Control).
 * El detalle se consulta solo mientras el modal está abierto. Foco atrapado y cierre con Escape los da Dialog.
 *
 * La alerta con que se abrió (`anchor`) da el punto y el grupo inicial; luego `alertId` cambia con las pestañas
 * sin reiniciar el modal (AlertDetailHost conserva la clave).
 */
export function AlertManagementDialog({ alertId, initialMode, onClose }: AlertManagementDialogProps) {
    const [anchorId] = useState(alertId);
    const anchorQuery = useAlertManagementDetail(anchorId);
    const anchor = anchorQuery.data;
    const popupRef = useRef<HTMLDivElement>(null);

    return (
        <Dialog open onOpenChange={(isOpen) => !isOpen && onClose()}>
            {/* El foco inicial va al modal y no al primer elemento enfocable (un tramo de la franja), que
                quedaba marcado con el anillo de foco al abrir. Tab sigue recorriendo desde el principio. */}
            <DialogContent ref={popupRef} initialFocus={popupRef} className={MANAGEMENT_DIALOG_CLASS}>
                {anchor ? (
                    <AlertGroupView anchor={anchor} activeId={alertId} initialMode={initialMode} onClose={onClose} />
                ) : (
                    <>
                        <DialogTitle className="sr-only">Gestión de alerta</DialogTitle>
                        <DetailLoadState query={anchorQuery} skeleton={<AlertDetailSkeleton />} />
                    </>
                )}
            </DialogContent>
        </Dialog>
    );
}
