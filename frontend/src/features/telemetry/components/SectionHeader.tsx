import { SECTION_STYLES, type SectionKey } from "../config/sectionStyles";

export function SectionHeader({ sectionKey }: { sectionKey: SectionKey }) {
    const { icon: Icon, label, subtitle, badgeClass } = SECTION_STYLES[sectionKey];
    return (
        <span className="flex items-center gap-3">
            <span className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border ${badgeClass}`}>
                <Icon size={18} />
            </span>
            <span className="flex flex-col">
                <span className="text-base font-bold text-slate-900">{label}</span>
                <span className="text-xs font-normal text-slate-500">{subtitle}</span>
            </span>
        </span>
    );
}