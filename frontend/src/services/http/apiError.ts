/** Código propio del frontend cuando la petición no llegó al servidor (sin red, CORS, servidor caído). */
export const NETWORK_ERROR_CODE = "NETWORK_ERROR";

interface ApiErrorInit {
    status: number;
    code: string;
    /** Texto apto para mostrar al usuario: el `message` del backend o un mensaje genérico en español. */
    message: string;
    body?: unknown;
}

/**
 * Error de una petición HTTP. El backend responde siempre `{ code, message }` con el mensaje en español,
 * así que `message` se puede mostrar tal cual; `body` conserva la respuesta para casos con datos (p. ej. 409).
 */
export class ApiError extends Error {
    readonly status: number;
    readonly code: string;
    readonly body: unknown;

    constructor({ status, code, message, body = null }: ApiErrorInit) {
        super(message);
        this.name = "ApiError";
        this.status = status;
        this.code = code;
        this.body = body;
    }
}

export function isApiError(error: unknown): error is ApiError {
    return error instanceof ApiError;
}

/** Mensaje para el usuario: el de un ApiError o el de respaldo. Nunca expone errores técnicos crudos. */
export function userErrorMessage(error: unknown, fallback: string): string {
    return isApiError(error) ? error.message : fallback;
}

/** Errores del cliente (4xx): reintentar no cambia el resultado. */
export function isClientError(error: unknown): boolean {
    return isApiError(error) && error.status >= 400 && error.status < 500;
}
