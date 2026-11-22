import type { AlertId, AlertItem } from "@/services/alertManagement";
import { Table, TableBody, TableCaption, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { cn } from "@/lib/utils";
import { SkeletonBlock } from "@/shared/components/Skeleton";
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

function HistoryTableHeader() {
    return (
        <TableHeader className="bg-tgi-verde/10">
            <TableRow className="hover:bg-transparent">
                {COLUMNS.map((column) => (
                    <TableHead
                        key={column}
                        scope="col"
                        className="px-3 font-data-mono text-xs font-bold tracking-wider text-gray-500 uppercase"
                    >
                        {column}
                    </TableHead>
                ))}
            </TableRow>
        </TableHeader>
    );
}

// Forma de cada celda de AlertHistoryRow, en el orden de COLUMNS: una o dos líneas, insignias y el botón de acción.
const SKELETON_CELLS: readonly string[][] = [
    ["h-3 w-20"],
    ["h-3 w-20", "h-3 w-12"],
    ["h-3 w-28", "h-3 w-24"],
    ["h-5 w-16 rounded-full"],
    ["h-5 w-16 rounded-full"],
    ["h-3 w-24"],
    ["h-5 w-20 rounded-full", "h-3 w-16"],
    ["h-3 w-16"],
    ["h-7 w-20 rounded-lg"],
];

/**
 * Tabla mientras carga la primera página: la cabecera real y filas con la forma de las de verdad. Va dentro de un
 * <Skeleton> (lo pone quien la usa, junto con el resto de la vista que carga).
 */
export function AlertHistoryTableSkeleton({ rows }: { rows: number }) {
    return (
        <Table>
            <HistoryTableHeader />
            <TableBody>
                {Array.from({ length: rows }, (_, row) => (
                    <TableRow key={row} className="hover:bg-transparent">
                        {SKELETON_CELLS.map((lines, cell) => (
                            <TableCell key={cell} className="px-3 py-3">
                                <div className="flex flex-col items-start gap-1.5">
                                    {lines.map((block) => (
                                        <SkeletonBlock key={block} className={block} />
                                    ))}
                                </div>
                            </TableCell>
                        ))}
                    </TableRow>
                ))}
            </TableBody>
        </Table>
    );
}

export function AlertHistoryTable({ items, caption, isUpdating, onAction, onIntent }: AlertHistoryTableProps) {
    return (
        <Table aria-busy={isUpdating} className={cn("transition-opacity", isUpdating && "opacity-60")}>
            <TableCaption className="sr-only">{caption}</TableCaption>
            <HistoryTableHeader />
            <TableBody>
                {items.map((item) => (
                    <AlertHistoryRow key={item.id} item={item} onAction={onAction} onIntent={onIntent} />
                ))}
            </TableBody>
        </Table>
    );
}
