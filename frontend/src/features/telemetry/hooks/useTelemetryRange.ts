import { useCallback, useMemo } from "react";
import { useSearchParams } from "react-router";
import { useNow } from "@/shared/hooks/useNow";
import { getRangeDates, type QuickRange } from "../lib/dateRanges";
import type { AppliedRange } from "../lib/customRange";
import { customSelectionFrom, customWindow, parseRangeSelection, withRangeSelection } from "../lib/rangeSearchParams";

/**
 * Rango de las gráficas, guardado en la URL: es el mismo en todas las pestañas y sobrevive a recargas.
 * Los rangos rápidos avanzan con el reloj compartido; como la ventana se redondea a 5 min, la query solo cambia
 * (y se piden datos nuevos) cuando cambia ese redondeo.
 */
export function useTelemetryRange() {
    const [searchParams, setSearchParams] = useSearchParams();
    const now = useNow();
    const selection = parseRangeSelection(searchParams, now);

    const { startDate, endDate } = selection.kind === "quick" ? getRangeDates(selection.range, now) : customWindow(selection, now);
    const range = useMemo(() => ({ startDate, endDate }), [startDate, endDate]);

    // replace: cambiar de rango no debe llenar el historial del navegador.
    const selectQuickRange = useCallback(
        (next: QuickRange) => setSearchParams((previous) => withRangeSelection(previous, { kind: "quick", range: next }), { replace: true }),
        [setSearchParams]
    );
    const selectCustomRange = useCallback(
        (next: AppliedRange) => setSearchParams((previous) => withRangeSelection(previous, customSelectionFrom(next)), { replace: true }),
        [setSearchParams]
    );

    return {
        quickRange: selection.kind === "quick" ? selection.range : null,
        customRange: selection.kind === "custom" ? range : null,
        ...range,
        selectQuickRange,
        selectCustomRange,
    };
}
