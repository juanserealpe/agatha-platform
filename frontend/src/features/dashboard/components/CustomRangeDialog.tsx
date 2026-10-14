// features/dashboard/CustomRangeDialog.tsx
import { useState } from "react";
import { DayPicker, type DateRange } from "react-day-picker";
import "react-day-picker/style.css";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";

interface CustomRangeDialogProps {
    onApply: (range: { startDate: string; endDate: string }) => void;
}

export function CustomRangeDialog({ onApply }: CustomRangeDialogProps) {
    const [range, setRange] = useState<DateRange | undefined>();
    const [open, setOpen] = useState(false);

    function handleApply() {
        if (!range?.from || !range?.to) return;
        onApply({ startDate: range.from.toISOString(), endDate: range.to.toISOString() });
        setOpen(false);
    }

    return (
        <Dialog open={open} onOpenChange={setOpen}>
            <DialogTrigger render={<Button variant="outline" className="font-data-mono text-sm" />}>
                Rango personalizado
            </DialogTrigger>
            <DialogContent className="sm:max-w-md max-h-[90vh] overflow-y-auto">
                <DialogHeader>
                    <DialogTitle>Selecciona un rango de fechas</DialogTitle>
                </DialogHeader>
                <DayPicker mode="range" numberOfMonths={1} selected={range} onSelect={setRange} />
                <Button onClick={handleApply} disabled={!range?.from || !range?.to}>
                    Aplicar
                </Button>
            </DialogContent>
        </Dialog>
    );
}