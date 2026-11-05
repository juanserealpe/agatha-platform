/** Descarga un archivo generado en memoria y libera su object URL. */
export function downloadBlob(blob: Blob, filename: string): void {
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = filename;
    link.rel = "noopener";
    link.style.display = "none";
    document.body.appendChild(link);
    try {
        link.click();
    } finally {
        link.remove();
        // Se libera en el siguiente ciclo: algunos navegadores leen la URL después del click.
        setTimeout(() => URL.revokeObjectURL(url), 0);
    }
}
