import { useNow } from "@/shared/hooks/useNow";
import { formatAbsolute, formatTimestamp, toIsoString, type DateInput } from "@/shared/lib/formatDate";

interface RelativeTimeProps {
    value: DateInput | null;
    fallback?: string;
    className?: string;
}

/** Muestra el tiempo relativo si es reciente y la fecha absoluta (24 h) en el tooltip. */
export function RelativeTime({ value, fallback = "Nunca", className }: RelativeTimeProps) {
    const now = useNow();

    if (value === null) {
        return <span className={className}>{fallback}</span>;
    }

    return (
        <time dateTime={toIsoString(value)} title={formatAbsolute(value)} className={className}>
            {formatTimestamp(value, now)}
        </time>
    );
}
