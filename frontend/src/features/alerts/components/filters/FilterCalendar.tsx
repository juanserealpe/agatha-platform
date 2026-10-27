import { DayPicker } from "react-day-picker";
import { es } from "react-day-picker/locale";
import "react-day-picker/style.css";

interface FilterCalendarProps {
    selected: Date | undefined;
    onSelect: (date: Date | undefined) => void;
}

// Archivo aparte para cargarlo con lazy(): el calendario solo se descarga al abrirlo.
export function FilterCalendar({ selected, onSelect }: FilterCalendarProps) {
    return (
        <DayPicker
            mode="single"
            locale={es}
            selected={selected}
            defaultMonth={selected}
            onSelect={onSelect}
            disabled={{ after: new Date() }}
        />
    );
}
