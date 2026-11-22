import { Suspense, useState } from "react";
import { CalendarDays } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { usePreloadWhenIdle } from "@/shared/hooks/usePreloadWhenIdle";
import { lazyWithPreload } from "@/shared/lib/lazyWithPreload";
import { DATE_PLACEHOLDER, dateToIsoDay, isoDayToDate, isoDayToDisplay, parseDisplayDay } from "../../lib/historyFilterForm";

// El calendario se descarga aparte y se precarga en tiempo ocioso o al acercarse al botón, para que al
// abrirlo ya esté listo (y, ya precargado, sin pasar por el fallback de Suspense: ver lazyWithPreload).
const { Component: FilterCalendar, preload: loadFilterCalendar } = lazyWithPreload(() =>
    import("./FilterCalendar").then((module) => module.FilterCalendar)
);
const preloadCalendar = () => void loadFilterCalendar();

// "dd/mm/aaaa" tiene 10 caracteres.
const DATE_TEXT_MAX_LENGTH = 10;

interface DateFilterFieldProps {
    id: string;
    label: string;
    /** Texto tal cual lo escribió el usuario. */
    value: string;
    onChange: (text: string) => void;
    invalid: boolean;
    errorId?: string;
}

/** Fecha en dd/mm/aaaa: se puede escribir o elegir en el calendario. */
export function DateFilterField({ id, label, value, onChange, invalid, errorId }: DateFilterFieldProps) {
    const [calendarOpen, setCalendarOpen] = useState(false);
    const isoDay = parseDisplayDay(value);
    usePreloadWhenIdle(loadFilterCalendar);

    function handleSelect(date: Date | undefined) {
        onChange(date ? isoDayToDisplay(dateToIsoDay(date)) : "");
        setCalendarOpen(false);
    }

    return (
        <div className="flex flex-col gap-1.5">
            <Label htmlFor={id} className="text-xs font-medium text-gray-600">
                {label}
            </Label>
            <div className="relative">
                <Input
                    id={id}
                    value={value}
                    onChange={(event) => onChange(event.target.value)}
                    placeholder={DATE_PLACEHOLDER}
                    inputMode="numeric"
                    autoComplete="off"
                    maxLength={DATE_TEXT_MAX_LENGTH}
                    aria-invalid={invalid || undefined}
                    aria-describedby={invalid ? errorId : undefined}
                    className="pr-9 font-data-mono"
                />
                <Popover open={calendarOpen} onOpenChange={setCalendarOpen}>
                    <PopoverTrigger
                        render={
                            <Button
                                variant="ghost"
                                size="icon-sm"
                                className="absolute top-1/2 right-0.5 -translate-y-1/2 text-gray-500"
                                aria-label={`Elegir ${label.toLowerCase()} en el calendario`}
                                onPointerEnter={preloadCalendar}
                                onFocus={preloadCalendar}
                            />
                        }
                    >
                        <CalendarDays aria-hidden />
                    </PopoverTrigger>
                    <PopoverContent align="end" className="w-auto">
                        <Suspense fallback={<p className="p-4 text-xs text-gray-500">Cargando calendario…</p>}>
                            <FilterCalendar selected={isoDay ? isoDayToDate(isoDay) : undefined} onSelect={handleSelect} />
                        </Suspense>
                    </PopoverContent>
                </Popover>
            </div>
        </div>
    );
}
