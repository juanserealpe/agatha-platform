import { cn } from "@/lib/utils";
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

/** Control segmentado de rangos rápidos (mismo material que las pestañas) y el botón del rango personalizado. */
export function DateRangeControls({ activeQuickRange, customRange, onQuickRangeSelect, onCustomRangeApply }: DateRangeControlsProps) {
    return (
        <div className="flex flex-wrap items-center gap-2">
            <div role="group" aria-label="Rango de tiempo" className="inline-flex h-8 items-center rounded-lg bg-slate-100 p-0.75">
                {QUICK_RANGE_OPTIONS.map((option) => {
                    const active = activeQuickRange === option.value;
                    return (
                        <button
                            key={option.value}
                            type="button"
                            aria-pressed={active}
                            onClick={() => onQuickRangeSelect(option.value)}
                            className={cn(
                                "h-full rounded-md px-2.5 text-xs font-medium whitespace-nowrap transition-colors",
                                "focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-1 focus-visible:outline-ring",
                                active ? "bg-white text-slate-900 shadow-sm" : "text-slate-600 hover:text-slate-900"
                            )}
                        >
                            {option.label}
                        </button>
                    );
                })}
            </div>
            <CustomRangeDialog applied={customRange} onApply={onCustomRangeApply} />
        </div>
    );
}
