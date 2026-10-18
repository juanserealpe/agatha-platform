import type { NivelAlerta } from "@/services/types";

export function getMarkerColor(nivel: NivelAlerta) {
    switch (nivel) {
        case "roja": return "#ef4444";
        case "amarilla": return "#f59e0b";
        default: return "#10b981";
    }
}
