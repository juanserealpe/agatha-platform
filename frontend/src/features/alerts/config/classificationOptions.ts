import type { LucideIcon } from "lucide-react";
import { CircleCheck, CircleX } from "lucide-react";
import type { Classification } from "@/services/alertManagement";

// Verde para confirmar y el rojo de acciones negativas (`destructive`) para descartar: no es la escala de
// severidad, y el ícono y el texto también comunican la opción.
export interface ClassificationOptionStyle {
    label: string;
    description: string;
    Icon: LucideIcon;
    /** Tarjeta: tinte suave sin elegir y más marcado al elegirla (además del radio, el ícono y el texto). */
    cardClass: string;
    radioClass: string;
}

// Verde para confirmar y el rojo de acciones negativas (`destructive`) para descartar: no es la escala de
// severidad, y el ícono y el texto también comunican la opción.
export const CLASSIFICATION_OPTIONS: Record<Classification, ClassificationOptionStyle> = {
    CONFIRMADA: {
        label: "Confirmar alerta real",
        description: "El evento ocurrió o se verificó en campo.",
        Icon: CircleCheck,
        cardClass:
            "border-emerald-200 bg-emerald-50/60 text-emerald-900 hover:bg-emerald-50 has-data-checked:border-emerald-600 has-data-checked:bg-emerald-100 has-data-checked:ring-1 has-data-checked:ring-emerald-600",
        radioClass: "data-checked:border-emerald-700 data-checked:bg-emerald-700",
    },
    FALSA_ALERTA: {
        label: "Falsa alerta",
        description: "No corresponde a un evento real en el ducto.",
        Icon: CircleX,
        cardClass:
            "border-destructive/25 bg-destructive/5 text-destructive hover:bg-destructive/10 has-data-checked:border-destructive has-data-checked:bg-destructive/10 has-data-checked:ring-1 has-data-checked:ring-destructive",
        radioClass: "data-checked:border-destructive data-checked:bg-destructive",
    },
};
