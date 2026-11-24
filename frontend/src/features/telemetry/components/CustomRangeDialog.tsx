import { useState } from "react";
import { DayPicker } from "react-day-picker";
import { es } from "react-day-picker/locale";
import "react-day-picker/style.css";
import { CalendarRange } from "lucide-react";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { formatDayMonth } from "@/shared/lib/formatDate";
import {
    EMPTY_DRAFT,
    draftFromApplied,
    isComplete,
    pickDay,
    toAppliedRange,
    type AppliedRange,
    type RangeDraft,
    type RangeEdge,
} from "../lib/customRange";
import { RangeEdgeButton } from "./RangeEdgeButton";

interface CustomRangeDialogProps {
    /** Rango personalizado vigente, o null si se está usando un rango rápido. */
    applied: AppliedRange | null;
    onApply: (range: AppliedRange) => void;
}

const EDGE_HINT: Record<RangeEdge, string> = {
    start: "Elige en el calendario la fecha de inicio.",
    end: "Elige en el calendario la fecha de fin.",
};

export function CustomRangeDialog({ applied, onApply }: CustomRangeDialogProps) {
    const [open, setOpen] = useState(false);
    const [draft, setDraft] = useState<RangeDraft>(EMPTY_DRAFT);
    const [edge, setEdge] = useState<RangeEdge>("start");
    const appliedDraft = draftFromApplied(applied);

    function handleOpenChange(next: boolean) {
        // Cada vez que se abre parte de lo que se está viendo, no de una edición a medias que se canceló.
        if (next) {
            setDraft(appliedDraft);
            setEdge("start");
        }
        setOpen(next);
    }

    function handleDayClick(day: Date) {
        const result = pickDay(draft, edge, day);
        setDraft(result.draft);
        setEdge(result.next);
    }

    function handleClear() {
        setDraft(EMPTY_DRAFT);
        setEdge("start");
    }

    function handleApply() {
        if (!isComplete(draft)) return;
        onApply(toAppliedRange(draft));
        setOpen(false);
    }

    const today = new Date();
    const complete = isComplete(draft);

    return (
        <Dialog open={open} onOpenChange={handleOpenChange}>
            <DialogTrigger
                render={<Button variant={applied ? "default" : "outline"} size="sm" className="text-xs" />}
            >
                <CalendarRange aria-hidden />
                {isComplete(appliedDraft)
                    ? `${formatDayMonth(appliedDraft.start)} – ${formatDayMonth(appliedDraft.end)}`
                    : "Rango personalizado"}
            </DialogTrigger>
            <DialogContent className="sm:max-w-sm max-h-[90vh] overflow-y-auto">
                <DialogHeader>
                    <DialogTitle>Rango personalizado</DialogTitle>
                    <DialogDescription>{EDGE_HINT[edge]}</DialogDescription>
                </DialogHeader>
                <div className="grid grid-cols-2 gap-2" role="group" aria-label="Extremos del rango">
                    <RangeEdgeButton
                        label="Inicio"
                        value={draft.start}
                        active={edge === "start"}
                        onClick={() => setEdge("start")}
                    />
                    <RangeEdgeButton label="Fin" value={draft.end} active={edge === "end"} onClick={() => setEdge("end")} />
                </div>
                <DayPicker
                    mode="range"
                    locale={es}
                    className="mx-auto"
                    selected={{ from: draft.start, to: draft.end }}
                    // Se ignora el rango que calcula DayPicker: el extremo a cambiar lo decide la casilla activa.
                    onSelect={(_range, day) => handleDayClick(day)}
                    defaultMonth={draft.start ?? draft.end ?? today}
                    endMonth={today}
                    disabled={{ after: today }}
                />
                <DialogFooter>
                    <Button variant="ghost" onClick={handleClear} disabled={!draft.start && !draft.end}>
                        Limpiar
                    </Button>
                    <Button onClick={handleApply} disabled={!complete}>
                        {complete ? `Aplicar ${formatDayMonth(draft.start)} – ${formatDayMonth(draft.end)}` : "Aplicar"}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}
