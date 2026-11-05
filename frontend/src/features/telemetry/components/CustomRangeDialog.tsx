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

        // Fin del día elegido, pero nunca después de "ahora": el backend rechaza fechas futuras
        const endOfDay = new Date(range.to);
        endOfDay.setHours(23, 59, 59, 999);
        const nowMinute = Math.floor(Date.now() / 60_000) * 60_000;
        const end = new Date(Math.min(endOfDay.getTime(), nowMinute));

        onApply({ startDate: range.from.toISOString(), endDate: end.toISOString() });
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
                <DayPicker mode="range" numberOfMonths={1} selected={range} onSelect={setRange} disabled={{ after: new Date() }} />
                <Button onClick={handleApply} disabled={!range?.from || !range?.to}>
                    Aplicar
                </Button>
            </DialogContent>
        </Dialog>
    );
}