const ENCODED_FILENAME = /filename\*\s*=\s*(?:UTF-8'')?("?)([^";]+)\1/i;
const PLAIN_FILENAME = /filename\s*=\s*(?:"([^"]*)"|([^;]+))/i;
const FIRST_PRINTABLE_CHAR_CODE = 32;

// El nombre viene del servidor, pero se trata como dato no confiable: sin rutas ni caracteres de control.
function sanitizeFilename(name: string): string | null {
    const clean = [...name]
        .filter((char) => char.charCodeAt(0) >= FIRST_PRINTABLE_CHAR_CODE)
        .join("")
        .replace(/[/\\]/g, "_")
        .trim();
    return clean === "" ? null : clean;
}

/** Nombre de archivo de una cabecera `Content-Disposition`, o null si no trae uno utilizable. */
export function parseContentDispositionFilename(header: string | null): string | null {
    if (!header) return null;

    const encoded = ENCODED_FILENAME.exec(header);
    if (encoded) {
        try {
            const name = sanitizeFilename(decodeURIComponent(encoded[2]));
            if (name) return name;
        } catch {
            // Codificación inválida: se intenta con el parámetro `filename` simple.
        }
    }

    const plain = PLAIN_FILENAME.exec(header);
    if (!plain) return null;
    return sanitizeFilename(plain[1] ?? plain[2] ?? "");
}
