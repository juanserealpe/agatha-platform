// features/dashboard/hooks/useDateRangeState.ts
import { useState } from "react";
import { DEFAULT_RANGE, getRangeDates, type QuickRange } from "../dateRanges";

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

    function selectQuickRange(range: QuickRange) {
        setState({ quickRange: range, ...getRangeDates(range) });
    }

    function selectCustomRange(range: { startDate: string; endDate: string }) {
        setState({ quickRange: null, ...range });
    }

    return { ...state, selectQuickRange, selectCustomRange };
}