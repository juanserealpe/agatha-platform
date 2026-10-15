import { useEffect, useState } from "react";
import { DEFAULT_RANGE, getRangeDates, type QuickRange } from "../lib/dateRanges";

interface DateRangeValue {
    quickRange: QuickRange | null;
    startDate: string;
    endDate: string;
}

export function useDateRangeState() {
    const [state, setState] = useState<DateRangeValue>(() => ({
        quickRange: DEFAULT_RANGE,
        ...getRangeDates(DEFAULT_RANGE),
    }));

    // Los rangos rápidos "avanzan" solos: cada minuto se recalcula la ventana y, si el
    // redondeo a 5 min cambió, la key de la query cambia y se piden datos nuevos.
    // Si no cambió, devolvemos `prev` y React no re-renderiza.
    useEffect(() => {
        const range = state.quickRange;
        if (range === null) return; // rango personalizado: es fijo, no se refresca
        const id = setInterval(() => {
            const next = getRangeDates(range);
            setState((prev) =>
                prev.quickRange === range && prev.endDate !== next.endDate
                    ? { quickRange: range, ...next }
                    : prev
            );
        }, 60_000);
        return () => clearInterval(id);
    }, [state.quickRange]);

    function selectQuickRange(range: QuickRange) {
        setState({ quickRange: range, ...getRangeDates(range) });
    }

    function selectCustomRange(range: { startDate: string; endDate: string }) {
        setState({ quickRange: null, ...range });
    }

    return { ...state, selectQuickRange, selectCustomRange };
}