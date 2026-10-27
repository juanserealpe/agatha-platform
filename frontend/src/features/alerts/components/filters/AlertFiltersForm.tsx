import { useId, useState, type FormEvent } from "react";
import { Search } from "lucide-react";
import type { AlertSearchFilters } from "@/services/alertManagement";
import { Button } from "@/components/ui/button";
import {
    EVENT_LABEL,
    EVENT_ORDER,
    LEVEL_CHIP_SELECTED_CLASS,
    LEVEL_ORDER,
    levelStyle,
    MANAGEMENT_STATUS_ORDER,
    MANAGEMENT_STATUS_STYLES,
} from "../../config/alertManagementDisplay";
import { draftFromFilters, validateDraft, type DateField, type FilterDraft } from "../../lib/historyFilterForm";
import { EMPTY_FILTERS } from "../../lib/historySearchParams";
import { DateFilterField } from "./DateFilterField";
import { DeviceFilterSelect } from "./DeviceFilterSelect";
import { FilterChips } from "./FilterChips";

interface AlertFiltersFormProps {
    /** Filtros de la última búsqueda: punto de partida de la edición. */
    applied: AlertSearchFilters;
    onApply: (filters: AlertSearchFilters) => void;
    onClear: () => void;
}

interface DateError {
    field: DateField;
    message: string;
}

/**
 * Filtros EN EDICIÓN. El estado es local a este formulario: escribir o elegir opciones no re-renderiza la
 * tabla, y nada se aplica hasta presionar "Buscar".
 */
export function AlertFiltersForm({ applied, onApply, onClear }: AlertFiltersFormProps) {
    const [draft, setDraft] = useState<FilterDraft>(() => draftFromFilters(applied));
    const [dateError, setDateError] = useState<DateError | null>(null);
    const ids = { device: useId(), from: useId(), to: useId(), error: useId() };

    function update<K extends keyof FilterDraft>(key: K, value: FilterDraft[K]) {
        setDraft((current) => ({ ...current, [key]: value }));
        if (key === "fromText" || key === "toText") setDateError(null);
    }

    function handleSubmit(event: FormEvent<HTMLFormElement>) {
        event.preventDefault();
        const result = validateDraft(draft);
        if (!result.ok) {
            setDateError({ field: result.field, message: result.message });
            return;
        }
        onApply(result.filters);
    }

    function handleClear() {
        setDraft(draftFromFilters(EMPTY_FILTERS));
        setDateError(null);
        onClear();
    }

    return (
        <form
            onSubmit={handleSubmit}
            noValidate
            aria-label="Filtros del historial"
            className="flex flex-col gap-4 rounded-xl border border-tgi-verde/15 bg-linear-to-b from-tgi-verde/5 to-white p-4 shadow-xs"
        >
            <div className="grid grid-cols-1 items-start gap-4 sm:grid-cols-3 xl:grid-cols-[minmax(180px,220px)_150px_150px_auto_auto]">
                <DeviceFilterSelect id={ids.device} value={draft.device} onChange={(device) => update("device", device)} />
                <DateFilterField
                    id={ids.from}
                    label="Fecha inicio"
                    value={draft.fromText}
                    onChange={(text) => update("fromText", text)}
                    invalid={dateError?.field === "from"}
                    errorId={ids.error}
                />
                <DateFilterField
                    id={ids.to}
                    label="Fecha fin"
                    value={draft.toText}
                    onChange={(text) => update("toText", text)}
                    invalid={dateError?.field === "to"}
                    errorId={ids.error}
                />
                <FilterChips
                    label="Nivel de criticidad"
                    options={LEVEL_ORDER}
                    getLabel={(level) => levelStyle(level).label}
                    getDotClass={(level) => levelStyle(level).dotClass}
                    getSelectedClass={(level) => LEVEL_CHIP_SELECTED_CLASS[level]}
                    value={draft.maxLevels}
                    onChange={(levels) => update("maxLevels", levels)}
                />
                <FilterChips
                    label="Evento"
                    options={EVENT_ORDER}
                    getLabel={(event) => EVENT_LABEL[event]}
                    value={draft.events}
                    onChange={(events) => update("events", events)}
                />
            </div>

            {dateError && (
                <p id={ids.error} role="alert" className="-mt-2 text-xs font-medium text-destructive">
                    {dateError.message}
                </p>
            )}

            <div className="flex flex-wrap items-end justify-between gap-4">
                <FilterChips
                    label="Estado"
                    options={MANAGEMENT_STATUS_ORDER}
                    getLabel={(status) => MANAGEMENT_STATUS_STYLES[status].label}
                    value={draft.statuses}
                    onChange={(statuses) => update("statuses", statuses)}
                />
                <div className="flex flex-wrap items-center gap-3">
                    <Button type="button" variant="ghost" onClick={handleClear}>
                        Limpiar filtros
                    </Button>
                    <Button type="submit">
                        <Search aria-hidden />
                        Buscar
                    </Button>
                </div>
            </div>
        </form>
    );
}
