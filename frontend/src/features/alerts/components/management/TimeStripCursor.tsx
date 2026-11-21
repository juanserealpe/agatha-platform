import { cn } from "@/lib/utils";
import type { StripCursor } from "../../lib/timeStrip";
import { TRACK_INSET_CLASS } from "./timeStripStyles";

/** Cursor de tiempo (§5.3): banda punteada sobre todas las barras en el rango bajo el cursor de la línea de tiempo. */
export function CursorBand({ cursor, className, style }: { cursor: StripCursor; className?: string; style?: React.CSSProperties }) {
    return (
        <div aria-hidden className={cn("pointer-events-none z-30", TRACK_INSET_CLASS, className)} style={style}>
            <div className="relative size-full">
                <div
                    className="absolute -inset-y-1 min-w-px border-x border-dashed border-gray-700 bg-gray-900/10"
                    style={{ left: `${cursor.offsetPct}%`, width: `${cursor.widthPct}%` }}
                />
            </div>
        </div>
    );
}
