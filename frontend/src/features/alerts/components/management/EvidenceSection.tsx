import { useId, useState, type ChangeEvent } from "react";
import { ImagePlus, X } from "lucide-react";
import { Button } from "@/components/ui/button";

const MAX_EVIDENCE_FILES = 5;

/**
 * Evidencias fotográficas (después del MVP). Solo se renderiza con ALERT_EVIDENCE_ENABLED: las fotos se eligen
 * localmente y NO se envían, porque todavía no existe el endpoint. Al crearse, el envío se conecta aquí.
 */
export function EvidenceSection() {
    const inputId = useId();
    const [files, setFiles] = useState<File[]>([]);

    function handleAdd(event: ChangeEvent<HTMLInputElement>) {
        const added = Array.from(event.target.files ?? []);
        setFiles((current) => [...current, ...added].slice(0, MAX_EVIDENCE_FILES));
        event.target.value = "";
    }

    return (
        <fieldset className="flex flex-col gap-2">
            <legend className="flex w-full items-baseline justify-between font-data-mono text-xs font-bold tracking-wider text-gray-600 uppercase">
                Evidencias fotográficas (opcional)
                <span className="font-normal">
                    {files.length} / {MAX_EVIDENCE_FILES} archivos
                </span>
            </legend>
            <ul className="flex flex-wrap gap-2">
                {files.map((file, index) => (
                    <li key={`${file.name}-${index}`} className="flex items-center gap-1 rounded-lg border border-border px-2 py-1 text-xs">
                        <span className="max-w-40 truncate">{file.name}</span>
                        <Button
                            type="button"
                            variant="ghost"
                            size="icon-xs"
                            aria-label={`Quitar ${file.name}`}
                            onClick={() => setFiles((current) => current.filter((_, i) => i !== index))}
                        >
                            <X aria-hidden />
                        </Button>
                    </li>
                ))}
                {files.length < MAX_EVIDENCE_FILES && (
                    <li>
                        <label
                            htmlFor={inputId}
                            className="flex cursor-pointer items-center gap-1.5 rounded-lg border border-dashed border-gray-300 px-3 py-1.5 text-xs text-gray-600 hover:bg-gray-50"
                        >
                            <ImagePlus className="size-3.5" aria-hidden />
                            Añadir foto
                        </label>
                        <input id={inputId} type="file" accept="image/*" multiple className="sr-only" onChange={handleAdd} />
                    </li>
                )}
            </ul>
        </fieldset>
    );
}
