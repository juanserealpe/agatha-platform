import { Radio, Wifi } from "lucide-react";

export function Header() {
    return (
        <header className="flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
            <div className="flex items-center gap-4">
                <div className="w-12 h-12 rounded-lg bg-blue-50 flex items-center justify-center border border-blue-200 text-blue-600 shadow-sm">
                    <Radio size={28} />
                </div>
                <div>
                    <h1 className="text-2xl font-bold text-gray-900 tracking-wide">
                        AGATHA
                    </h1>
                    <p className="font-data-mono text-gray-500 text-xs mt-1 uppercase tracking-wider">
                        Monitoreo en tiempo real · Gasoducto Otero-Santana
                    </p>
                </div>
            </div>

            <div className="flex items-center gap-6">
                <div className="font-data-mono text-xs text-gray-600 bg-gray-50 px-3 py-1.5 rounded border border-gray-200">
                    <span className="text-gray-400 mr-1">Sitio:</span>
                    Güepsa - San José de Pare
                </div>
                <div className="flex items-center gap-2 bg-emerald-50 border border-emerald-200 px-3 py-1.5 rounded-full text-emerald-700 text-sm font-medium shadow-sm">
                    <Wifi size={16} />
                    <span className="font-data-mono text-xs uppercase">Conectado</span>
                </div>
            </div>
        </header>
    );
}