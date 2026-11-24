import type { ReactNode } from "react";
import { CircleAlert, CircleCheck, Info } from "lucide-react";
import { cn } from "@/lib/utils";

export type NoticeTone = "success" | "info" | "error";

// Sin amarillo/naranja/rojo de severidad: el aviso de conflicto usa el azul de marca y el error el token
// `destructive` de la app.
const TONE_STYLES: Record<NoticeTone, { className: string; Icon: typeof Info }> = {
    success: { className: "border-emerald-200 bg-emerald-50 text-emerald-800", Icon: CircleCheck },
    info: { className: "border-tgi-azul/25 bg-tgi-azul/5 text-tgi-azul-oscuro", Icon: Info },
    error: { className: "border-destructive/30 bg-destructive/5 text-destructive", Icon: CircleAlert },
};

interface ManagementNoticeProps {
    tone: NoticeTone;
    children: ReactNode;
    className?: string;
}

/** Aviso dentro del modal. Los errores se anuncian de inmediato; el resto, sin interrumpir. */
export function ManagementNotice({ tone, children, className }: ManagementNoticeProps) {
    const { className: toneClass, Icon } = TONE_STYLES[tone];
    return (
        <div
            role={tone === "error" ? "alert" : "status"}
            className={cn("flex items-start gap-2 rounded-lg border px-3 py-2 text-sm", toneClass, className)}
        >
            <Icon className="mt-0.5 size-4 shrink-0" aria-hidden />
            {/* Puede citar la etiqueta "Otro" escrita por el usuario. */}
            <div className="min-w-0 wrap-anywhere">{children}</div>
        </div>
    );
}
