// features/dashboard/DateRangeControls.tsx
import { Button } from "@/components/ui/button";
import { CustomRangeDialog } from "./CustomRangeDialog";
import { QUICK_RANGE_OPTIONS, type QuickRange } from "../lib/dateRanges";

interface DateRangeControlsProps {
    activeQuickRange: QuickRange | null;
    onQuickRangeSelect: (range: QuickRange) => void;
    onCustomRangeApply: (range: { startDate: string; endDate: string }) => void;
}

export function DateRangeControls({
    activeQuickRange,
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
            <CustomRangeDialog onApply={onCustomRangeApply} />
        </div>
    );
}