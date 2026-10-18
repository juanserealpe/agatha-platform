import { useState, type ReactNode } from "react";
import { ChevronDown } from "lucide-react";
import type { PathOptions } from "leaflet";
import { FLEXIBLE_MASK_STYLE, LEGEND_TONES, NODE_TONE_STYLES, PIPE_CASING_STYLE, PIPE_STYLES, WARNING_HEX, WATER_STYLES } from "../config/mapStyles";

function Dot({ color, warning = false, blink = false }: { color: string; warning?: boolean; blink?: boolean }) {
    return (
        <span
            className={`relative inline-block h-3 w-3 shrink-0 rounded-full border-2 border-white shadow ${blink ? "map-node--blink" : ""}`}
            style={{ background: color }}
        >
            {warning && (
                <span
                    className="absolute -right-1 -top-1 h-2 w-2 rounded-full border border-white"
                    style={{ background: WARNING_HEX }}
                />
            )}
        </span>
    );
}

/** Muestra de línea dibujada con los mismos estilos que el mapa (casing + trazo). */
function LineSwatch({ layers }: { layers: PathOptions[] }) {
    return (
        <svg width="24" height="10" viewBox="0 0 24 10" className="shrink-0 rounded-sm bg-slate-500" aria-hidden>
            {layers.map((layer, i) => (
                <line
                    key={i}
                    x1="2" y1="5" x2="22" y2="5"
                    stroke={layer.color}
                    strokeWidth={Math.min(layer.weight ?? 2, 6)}
                    strokeOpacity={layer.opacity ?? 1}
                    strokeDasharray={layer.dashArray as string | undefined}
                    strokeLinecap="round"
                />
            ))}
        </svg>
    );
}

function Row({ symbol, children }: { symbol: ReactNode; children: ReactNode }) {
    return (
        <li className="flex items-center gap-2">
            <span className="flex w-6 justify-center">{symbol}</span>
            <span>{children}</span>
        </li>
    );
}

export function MapLegend() {
    const [isExpanded, setIsExpanded] = useState(true);

    return (
        <div className="absolute bottom-6 right-3 z-[1000] w-60 overflow-hidden rounded-lg bg-white/95 text-xs text-gray-700 shadow-lg backdrop-blur">
            <button
                type="button"
                onClick={() => setIsExpanded((value) => !value)}
                aria-expanded={isExpanded}
                className="flex w-full items-center px-3 py-1.5 font-medium hover:bg-gray-50"
            >
                Leyenda
                <ChevronDown size={14} className={`ml-auto transition-transform ${isExpanded ? "" : "rotate-180"}`} aria-hidden />
            </button>

            {isExpanded && (
                <div className="flex flex-col gap-3 border-t border-gray-200 px-3 py-2.5">
                    <section>
                        <h3 className="mb-1.5 font-semibold text-gray-900">Estado del punto</h3>
                        <ul className="flex flex-col gap-1.5">
                            {LEGEND_TONES.map((tone) => (
                                <Row key={tone} symbol={<Dot color={NODE_TONE_STYLES[tone].hex} blink={tone === "red"} />}>
                                    {NODE_TONE_STYLES[tone].label}
                                    {tone === "red" && <span className="text-gray-500"> (titila mientras esté sin clasificar)</span>}
                                </Row>
                            ))}
                            <Row symbol={<Dot color={NODE_TONE_STYLES.orange.hex} warning />}>
                                Alerta + aviso <span className="text-gray-500">(conserva el color de la alerta)</span>
                            </Row>
                        </ul>
                    </section>

                    <section>
                        <h3 className="mb-1.5 font-semibold text-gray-900">Tubería</h3>
                        <ul className="flex flex-col gap-1.5">
                            <Row symbol={<LineSwatch layers={[PIPE_CASING_STYLE, PIPE_STYLES.steel.path]} />}>{PIPE_STYLES.steel.label}</Row>
                            <Row symbol={<LineSwatch layers={[FLEXIBLE_MASK_STYLE, PIPE_STYLES.flexible.path]} />}>{PIPE_STYLES.flexible.label}</Row>
                            <Row symbol={<LineSwatch layers={[WATER_STYLES.river.path]} />}>{WATER_STYLES.river.label}</Row>
                            <Row symbol={<LineSwatch layers={[WATER_STYLES.stream.path]} />}>{WATER_STYLES.stream.label}</Row>
                        </ul>
                    </section>
                </div>
            )}
        </div>
    );
}
