import { cn } from "@/lib/utils";
import { levelStyle } from "../../config/alertManagementDisplay";
import type { StripPiece } from "../../lib/timeStrip";

// Clases, medidas y posiciones compartidas por la franja expandida y la compacta.

// Tramo sin alerta (§2.1: #E5E7EB): gris neutro, fuera de la escala de severidad.
const IDLE_PIECE_CLASS = "bg-gray-200";
// Margen lateral para que los círculos del inicio y de "Ahora" no se corten; el eje usa el mismo.
export const TRACK_INSET_CLASS = "px-1.5";
export const FOCUS_CLASS = "outline-none focus-visible:ring-2 focus-visible:ring-gray-900 focus-visible:ring-offset-1";
// Tramo elegido desde la franja (§5.3, paso 4): contorno oscuro que se mantiene hasta elegir otro.
export const SELECTED_PIECE_CLASS = "z-10 ring-2 ring-gray-900 ring-offset-1";

// Cada fila (eje y barras) es su propia grilla con la columna de nombres de ancho FIJO: así las pistas de todas
// las filas quedan alineadas con el eje sin depender del texto. En celular, la expandida apila nombre, barra y
// fechas; la compacta deja solo el ícono. Las capas superpuestas (líneas del eje, "Ahora", cursor de tiempo)
// se ubican con el mismo desplazamiento: relleno de la fila + columna + separación.
export const ROW_PAD_CLASS = "px-2";
export const EXPANDED_ROW_GRID_CLASS = "sm:grid sm:grid-cols-[13rem_minmax(0,1fr)] sm:items-center sm:gap-x-3";
export const COMPACT_ROW_GRID_CLASS = "grid grid-cols-[1rem_minmax(0,1fr)] items-center gap-x-3 sm:grid-cols-[13rem_minmax(0,1fr)]";
export const EXPANDED_OVERLAY_X_CLASS = "left-2 right-2 sm:left-[calc(0.5rem+13rem+0.75rem)]";
export const COMPACT_OVERLAY_X_CLASS = "left-[calc(0.5rem+1rem+0.75rem)] right-2 sm:left-[calc(0.5rem+13rem+0.75rem)]";

// Jerarquía con varias alertas (pedido del usuario, 21-nov): la abierta con fondo neutro; las otras con la barra
// atenuada. Solo la barra: el texto queda en gris normal para no perder contraste (opacidad sobre texto no pasa AA).
export const CURRENT_ROW_CLASS = "rounded-md bg-muted";
export const DIMMED_TRACK_CLASS = "opacity-55";

export function pieceColorClass(piece: StripPiece): string {
    return piece.level ? levelStyle(piece.level).dotClass : IDLE_PIECE_CLASS;
}

export function piecePosition(piece: StripPiece) {
    return { left: `${piece.offsetPct}%`, width: `${piece.widthPct}%` };
}

export function edgeRounding(index: number, count: number): string {
    return cn(index === 0 && "rounded-l-full", index === count - 1 && "rounded-r-full");
}

/**
 * Corte de 2 px entre dos tramos de color seguidos: separa los niveles aunque la barra sea delgada. Es un borde del
 * color del fondo de la fila (`cutColorClass`: el del panel o el de la fila resaltada).
 */
export function levelCutClass(pieces: readonly StripPiece[], index: number, cutColorClass: string): string {
    const piece = pieces[index];
    const previous = pieces[index - 1];
    const follows = piece.kind === "level" && previous?.kind === "level" && previous.endMs >= piece.startMs;
    return follows ? cn("border-l-2", cutColorClass) : "";
}
