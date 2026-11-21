import { cn } from "@/lib/utils";
import { levelStyle } from "../../config/alertManagementDisplay";
import type { StripPiece } from "../../lib/timeStrip";

// Clases y posiciones compartidas por la franja expandida y la compacta.

// Tramo sin alerta (§2.1: #E5E7EB): gris neutro, fuera de la escala de severidad.
const IDLE_PIECE_CLASS = "bg-gray-200";
// Margen lateral para que los círculos del inicio y de "Ahora" no se corten; el eje usa el mismo.
export const TRACK_INSET_CLASS = "px-1.5";
export const FOCUS_CLASS = "outline-none focus-visible:ring-2 focus-visible:ring-gray-900 focus-visible:ring-offset-1";
// Tramo elegido desde la franja (§5.3, paso 4): contorno oscuro que se mantiene hasta elegir otro.
export const SELECTED_PIECE_CLASS = "z-10 ring-2 ring-gray-900 ring-offset-1";

export function pieceColorClass(piece: StripPiece): string {
    return piece.level ? levelStyle(piece.level).dotClass : IDLE_PIECE_CLASS;
}

export function piecePosition(piece: StripPiece) {
    return { left: `${piece.offsetPct}%`, width: `${piece.widthPct}%` };
}

export function edgeRounding(index: number, count: number): string {
    return cn(index === 0 && "rounded-l-full", index === count - 1 && "rounded-r-full");
}
