import { useState } from "react";
import { Download, Loader2 } from "lucide-react";
import type { AlertSearchFilters } from "@/services/alertManagement";
import { userErrorMessage } from "@/services/http";
import { Button } from "@/components/ui/button";
import { useExportAlerts } from "../../hooks/useExportAlerts";
import { EXPORT_TOO_LARGE_MESSAGE, exceedsExportLimit } from "../../lib/exportFile";

const EXPORT_ERROR = "No fue posible exportar las alertas. Intente nuevamente.";

interface ExportAlertsButtonProps {
    /** Filtros APLICADOS: se exporta exactamente lo que muestra la tabla, en todas sus páginas. */
    filters: AlertSearchFilters;
    total: number;
}

export function ExportAlertsButton({ filters, total }: ExportAlertsButtonProps) {
    const exportAlerts = useExportAlerts();
    const [limitExceeded, setLimitExceeded] = useState(false);

    function handleExport() {
        // Se conoce el total: no tiene sentido pedir un archivo que el backend rechazará.
        if (exceedsExportLimit(total)) {
            setLimitExceeded(true);
            return;
        }
        setLimitExceeded(false);
        exportAlerts.mutate(filters);
    }

    const errorMessage = limitExceeded
        ? EXPORT_TOO_LARGE_MESSAGE
        : exportAlerts.isError
            ? userErrorMessage(exportAlerts.error, EXPORT_ERROR)
            : null;

    return (
        <div className="flex flex-col items-end gap-1">
            {/* Azul TGI oscuro: buen contraste con texto blanco y sin significado de severidad. */}
            <Button
                onClick={handleExport}
                disabled={total === 0 || exportAlerts.isPending}
                className="bg-tgi-azul-oscuro text-white hover:bg-tgi-azul-oscuro/90"
            >
                {exportAlerts.isPending ? <Loader2 className="animate-spin" aria-hidden /> : <Download aria-hidden />}
                {exportAlerts.isPending ? "Exportando…" : "Exportar Excel"}
            </Button>
            {/* Región siempre presente para que el lector de pantalla anuncie el error al aparecer. */}
            <p aria-live="assertive" className="max-w-xs text-right text-xs font-medium text-destructive">
                {errorMessage}
            </p>
        </div>
    );
}
