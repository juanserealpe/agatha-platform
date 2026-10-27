import type { AlertId, AlertItem } from "@/services/alertManagement";
import { Table, TableBody, TableCaption, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { cn } from "@/lib/utils";
import type { RowAction } from "../../lib/rowActions";
import { AlertHistoryRow } from "./AlertHistoryRow";

// Algunos datos relacionados comparten celda en dos líneas (dispositivo y PK, inicio y fin, estado y
// clasificación) para que la tabla quepa sin desplazamiento horizontal en pantallas de escritorio.
const COLUMNS = [
    "ID alerta",
    "Dispositivo",
    "Inicio / Fin",
    "Nivel actual",
    "Nivel máximo",
    "Evento",
    "Estado",
    "Etiqueta",
    "Acción",
] as const;

interface AlertHistoryTableProps {
    items: AlertItem[];
    caption: string;
    /** Hay una página nueva en camino: se atenúa la actual en lugar de vaciarla. */
    isUpdating: boolean;
    onAction: (id: AlertId, action: RowAction) => void;
    onIntent: (id: AlertId) => void;
}

export function AlertHistoryTable({ items, caption, isUpdating, onAction, onIntent }: AlertHistoryTableProps) {
    return (
        <Table aria-busy={isUpdating} className={cn("transition-opacity", isUpdating && "opacity-60")}>
            <TableCaption className="sr-only">{caption}</TableCaption>
            <TableHeader className="bg-tgi-verde/10">
                <TableRow className="hover:bg-transparent">
                    {COLUMNS.map((column, index) => (
                        <TableHead
                            key={column}
                            scope="col"
                            className={cn(
                                "px-3 font-data-mono text-xs font-bold tracking-wider text-gray-500 uppercase",
                                // Alinea el encabezado con el borde de resaltado de la primera celda.
                                index === 0 && "border-l-4 border-l-transparent"
                            )}
                        >
                            {column}
                        </TableHead>
                    ))}
                </TableRow>
            </TableHeader>
            <TableBody>
                {items.map((item) => (
                    <AlertHistoryRow key={item.id} item={item} onAction={onAction} onIntent={onIntent} />
                ))}
            </TableBody>
        </Table>
    );
}
