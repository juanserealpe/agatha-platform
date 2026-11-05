import { ApiError, NETWORK_ERROR_CODE } from "./apiError";
import { parseContentDispositionFilename } from "./contentDisposition";

// En desarrollo Vite hace proxy de /api al backend; en producción la URL base viene solo del entorno.
const BASE_URL = import.meta.env.DEV ? "" : (import.meta.env.VITE_API_BASE_URL ?? "");

const NETWORK_ERROR_MESSAGE = "No fue posible conectar con el servidor. Intente nuevamente.";
const DEFAULT_ERROR_MESSAGE = "Ocurrió un error inesperado. Intente nuevamente.";

type QueryScalar = string | number;
type QueryValue = QueryScalar | readonly QueryScalar[] | null | undefined;
export type QueryParams = Record<string, QueryValue>;

export interface RequestOptions {
    query?: QueryParams;
    signal?: AbortSignal;
    /** Mensaje para el usuario cuando el backend no envía uno propio. */
    errorMessage?: string;
}

export interface HttpResponse<T> {
    status: number;
    data: T;
}

export interface BlobResponse {
    blob: Blob;
    /** Nombre de `Content-Disposition`; null si no viene o el navegador no lo expone (CORS). */
    filename: string | null;
}

/** Ruta de la API con cada segmento codificado, p. ej. `apiPath("api", "alerts", id)`. */
export function apiPath(...segments: QueryScalar[]): string {
    return `/${segments.map((segment) => encodeURIComponent(String(segment))).join("/")}`;
}

/** URL completa de una ruta de la API, con la misma base que `httpClient` (p. ej. para `EventSource`). */
export function apiUrl(path: string, query?: QueryParams): string {
    return `${BASE_URL}${path}${buildQueryString(query)}`;
}

function isList(value: QueryValue): value is readonly QueryScalar[] {
    return Array.isArray(value);
}

/** Query string sin valores vacíos; los arreglos repiten el parámetro (`a=1&a=2`). */
export function buildQueryString(query: QueryParams = {}): string {
    const params = new URLSearchParams();
    for (const [key, value] of Object.entries(query)) {
        if (value === null || value === undefined || value === "") continue;
        if (isList(value)) {
            value.forEach((item) => params.append(key, String(item)));
        } else {
            params.append(key, String(value));
        }
    }
    const serialized = params.toString();
    return serialized === "" ? "" : `?${serialized}`;
}

// Único punto donde se arman las cabeceras: aquí se agregará el token cuando exista autenticación (RN-10).
function buildHeaders(hasBody: boolean): HeadersInit {
    return hasBody ? { Accept: "application/json", "Content-Type": "application/json" } : { Accept: "application/json" };
}

function isRecord(value: unknown): value is Record<string, unknown> {
    return typeof value === "object" && value !== null;
}

async function readJson(response: Response): Promise<unknown> {
    try {
        return await response.json();
    } catch {
        return null;
    }
}

async function toApiError(response: Response, fallbackMessage = DEFAULT_ERROR_MESSAGE): Promise<ApiError> {
    const body = await readJson(response);
    const code = isRecord(body) && typeof body.code === "string" ? body.code : `HTTP_${response.status}`;
    const message = isRecord(body) && typeof body.message === "string" && body.message.trim() !== ""
        ? body.message
        : fallbackMessage;
    return new ApiError({ status: response.status, code, message, body });
}

/** Código del error cuando las peticiones al backend están bloqueadas (modo demo). */
export const BACKEND_BLOCKED_CODE = "BACKEND_BLOCKED";

let blockedMessage: string | null = null;

/**
 * Rechaza toda petición desde aquí en adelante, sin llegar a la red. La demo lo activa para que nunca llame
 * al backend real: si un servicio quedó sin simular, falla a la vista en vez de traer datos reales.
 */
export function blockBackendRequests(message: string) {
    blockedMessage = message;
}

async function send(method: "GET" | "POST", path: string, options: RequestOptions, body?: unknown): Promise<Response> {
    if (blockedMessage !== null) {
        console.error(`Petición bloqueada: ${method} ${path}`);
        throw new ApiError({ status: 0, code: BACKEND_BLOCKED_CODE, message: blockedMessage });
    }

    let response: Response;
    try {
        response = await fetch(apiUrl(path, options.query), {
            method,
            headers: buildHeaders(body !== undefined),
            body: body === undefined ? undefined : JSON.stringify(body),
            signal: options.signal,
        });
    } catch (error) {
        // Una cancelación (TanStack Query descarta la petición) no es un error que deba ver el usuario.
        if (options.signal?.aborted) throw error;
        throw new ApiError({ status: 0, code: NETWORK_ERROR_CODE, message: NETWORK_ERROR_MESSAGE });
    }

    if (!response.ok) throw await toApiError(response, options.errorMessage);
    return response;
}

/**
 * Cliente HTTP único de la app. Las respuestas se tipan según el contrato del backend: el cliente no las
 * valida en tiempo de ejecución, por eso cada servicio declara el tipo exacto que documenta su endpoint.
 */
export const httpClient = {
    async get<T>(path: string, options: RequestOptions = {}): Promise<T> {
        const response = await send("GET", path, options);
        return (await response.json()) as T;
    },

    async post<T>(path: string, body: unknown, options: RequestOptions = {}): Promise<HttpResponse<T>> {
        const response = await send("POST", path, options, body);
        return { status: response.status, data: (await response.json()) as T };
    },

    async getBlob(path: string, options: RequestOptions = {}): Promise<BlobResponse> {
        const response = await send("GET", path, options);
        return {
            blob: await response.blob(),
            filename: parseContentDispositionFilename(response.headers.get("Content-Disposition")),
        };
    },
};
