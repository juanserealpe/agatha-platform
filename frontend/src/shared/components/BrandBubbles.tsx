import { cn } from "@/lib/utils";

interface Bubble {
    cx: number;
    cy: number;
    r: number;
    className: string;
}

// Composición inspirada en los círculos del logo de TGI (no es el logo): tamaños y colores de marca
// muy translúcidos, pensada para asomarse recortada por el borde de un contenedor.
const BUBBLES: readonly Bubble[] = [
    { cx: 150, cy: 34, r: 46, className: "fill-tgi-verde/20" },
    { cx: 92, cy: 70, r: 26, className: "fill-tgi-lima/30" },
    { cx: 196, cy: 92, r: 30, className: "fill-tgi-celeste/20" },
    { cx: 60, cy: 22, r: 10, className: "fill-tgi-celeste/30" },
    { cx: 124, cy: 104, r: 9, className: "fill-tgi-azul/20" },
    { cx: 36, cy: 78, r: 6, className: "fill-tgi-verde/30" },
    { cx: 214, cy: 18, r: 12, className: "fill-tgi-lima/25" },
];

interface BrandBubblesProps {
    className?: string;
}

/** Decoración de identidad TGI. Puramente visual: no se anuncia a lectores de pantalla. */
export function BrandBubbles({ className }: BrandBubblesProps) {
    return (
        <svg
            viewBox="0 0 240 120"
            aria-hidden
            focusable="false"
            className={cn("pointer-events-none select-none", className)}
        >
            {BUBBLES.map((bubble) => (
                <circle key={`${bubble.cx}-${bubble.cy}`} cx={bubble.cx} cy={bubble.cy} r={bubble.r} className={bubble.className} />
            ))}
        </svg>
    );
}
