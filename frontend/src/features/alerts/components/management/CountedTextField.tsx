import { useId } from "react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { cn } from "@/lib/utils";

interface CountedTextFieldProps {
    label: string;
    value: string;
    onChange: (value: string) => void;
    /** El campo no admite más caracteres que este límite. */
    maxLength: number;
    multiline?: boolean;
    required?: boolean;
    invalid?: boolean;
    placeholder?: string;
    autoFocus?: boolean;
    className?: string;
}

/** Campo de texto con contador "N / máx." visible y anunciado junto al campo. */
export function CountedTextField({
    label,
    value,
    onChange,
    maxLength,
    multiline = false,
    required = false,
    invalid = false,
    placeholder,
    autoFocus,
    className,
}: CountedTextFieldProps) {
    const id = useId();
    const counterId = `${id}-counter`;
    const fieldProps = {
        id,
        value,
        maxLength,
        placeholder,
        autoFocus,
        required,
        "aria-required": required || undefined,
        "aria-invalid": invalid || undefined,
        "aria-describedby": counterId,
    };

    return (
        <div className={cn("flex flex-col gap-1.5", className)}>
            <div className="flex items-baseline justify-between gap-2">
                <Label htmlFor={id} className="font-data-mono text-xs font-bold tracking-wider text-gray-600 uppercase">
                    {label}
                    {required && <span className="text-destructive" aria-hidden> *</span>}
                </Label>
                <span id={counterId} className="font-data-mono text-xs text-gray-500">
                    {value.length} / {maxLength}
                    <span className="sr-only"> caracteres</span>
                </span>
            </div>
            {multiline ? (
                <Textarea {...fieldProps} rows={3} onChange={(event) => onChange(event.target.value)} />
            ) : (
                <Input {...fieldProps} onChange={(event) => onChange(event.target.value)} />
            )}
        </div>
    );
}
