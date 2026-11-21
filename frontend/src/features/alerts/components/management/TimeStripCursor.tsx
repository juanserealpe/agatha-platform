import type { StripCursor } from "../../lib/timeStrip";

/** Cursor de tiempo (§5.3): banda punteada sobre todas las barras en el rango bajo el cursor de la línea de tiempo. */
export function CursorBand({ cursor }: { cursor: StripCursor }) {
    return (
        <div
            className="absolute -inset-y-1 z-30 min-w-px border-x border-dashed border-gray-700 bg-gray-900/10"
            style={{ left: `${cursor.offsetPct}%`, width: `${cursor.widthPct}%` }}
        />
    );
}
