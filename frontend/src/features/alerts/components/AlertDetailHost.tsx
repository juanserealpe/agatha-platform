import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { useAlert } from "../hooks/useAlerts";
import { useAlertDetailParam } from "../hooks/useAlertDetailParam";
import { ALERT_TYPE_LABEL, SEVERITY_STYLES } from "../config/alertDisplay";

/**
 * Punto de entrada del detalle de alerta. Se monta una sola vez en el layout raíz y se abre con ?alerta=<id>.
 * El contenido del modal (clasificación, observaciones) se implementará en una fase posterior.
 */
export function AlertDetailHost() {
    const { alertId, close } = useAlertDetailParam();
    const { data: alert, isLoading, error } = useAlert(alertId);

    if (alertId === null) return null;

    const title = alert
        ? `${SEVERITY_STYLES[alert.severity].label} · ${ALERT_TYPE_LABEL[alert.type]}`
        : "Detalle de alerta";

    const description = isLoading
        ? "Cargando alerta…"
        : error
            ? "No se pudo cargar la alerta."
            : alert
                ? "El detalle y la clasificación de la alerta están en construcción."
                : `No se encontró la alerta «${alertId}».`;

    return (
        <Dialog open onOpenChange={(isOpen) => !isOpen && close()}>
            <DialogContent className="sm:max-w-md">
                <DialogHeader>
                    <DialogTitle>{title}</DialogTitle>
                    <DialogDescription>{description}</DialogDescription>
                </DialogHeader>
            </DialogContent>
        </Dialog>
    );
}
