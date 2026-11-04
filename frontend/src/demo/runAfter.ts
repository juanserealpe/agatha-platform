// Temporizador para "Disparar en 5 s". Con el navegador minimizado, Chrome y Windows frenan los temporizadores de
// la página (pueden retrasarse minutos), y el paso no ocurría hasta volver a la ventana. Los de un Web Worker no
// sufren ese freno y su mensaje despierta a la página, así que el aviso llega a tiempo.

const WORKER_SOURCE = "onmessage = (event) => setTimeout(() => postMessage(null), event.data);";

/** Ejecuta `callback` tras `delayMs` y devuelve cómo cancelarlo. Sin Worker, usa `setTimeout` de la página. */
export function runAfter(delayMs: number, callback: () => void): () => void {
    let worker: Worker | null = null;
    let url: string | null = null;
    try {
        url = URL.createObjectURL(new Blob([WORKER_SOURCE], { type: "text/javascript" }));
        worker = new Worker(url);
    } catch {
        if (url) URL.revokeObjectURL(url);
        const timer = setTimeout(callback, delayMs);
        return () => clearTimeout(timer);
    }

    const release = () => {
        worker?.terminate();
        if (url) URL.revokeObjectURL(url);
    };
    worker.onmessage = () => {
        release();
        callback();
    };
    worker.postMessage(delayMs);
    return release;
}
