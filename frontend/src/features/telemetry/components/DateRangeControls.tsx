import { Button } from "@/components/ui/button";
import { CustomRangeDialog } from "./CustomRangeDialog";
import { QUICK_RANGE_OPTIONS, type QuickRange } from "../lib/dateRanges";
import type { AppliedRange } from "../lib/customRange";

interface DateRangeControlsProps {
    activeQuickRange: QuickRange | null;
    /** Rango personalizado vigente, o null si hay un rango rápido activo. */
    customRange: AppliedRange | null;
    onQuickRangeSelect: (range: QuickRange) => void;
    onCustomRangeApply: (range: AppliedRange) => void;
}

export function DateRangeControls({
    activeQuickRange,
    customRange,
    onQuickRangeSelect,
    onCustomRangeApply,
}: DateRangeControlsProps) {
    return (
        <div className="flex flex-wrap gap-2 items-center">
            {QUICK_RANGE_OPTIONS.map((opt) => (
                <Button
                    key={opt.value}
                    variant={activeQuickRange === opt.value ? "default" : "outline"}
                    size="sm"
                    className="font-data-mono text-xs"
                    onClick={() => onQuickRangeSelect(opt.value)}
                >
                    {opt.label}
                </Button>
            ))}
            <CustomRangeDialog applied={customRange} onApply={onCustomRangeApply} />
        </div>
    );
}