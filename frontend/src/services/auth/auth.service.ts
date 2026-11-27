import { apiPath, httpClient, isApiError, userErrorMessage } from "@/services/http";
import { tokenStore, type StoredSession } from "./tokenStore";

interface LoginResponse {
    accessToken: string;
    tokenType: "Bearer";
    expiresAt: string;
    username: string;
}

const LOGIN_ERROR_MESSAGE = "No fue posible iniciar sesión.";

export async function login(username: string, password: string): Promise<StoredSession> {
    // El backend rechaza con 400 cualquier campo extra: solo `username` y `password`.
    const { data } = await httpClient.post<LoginResponse>(
        apiPath("api", "auth", "login"),
        { username, password },
        { errorMessage: LOGIN_ERROR_MESSAGE },
    );
    const session = { accessToken: data.accessToken, expiresAt: data.expiresAt, username: data.username };
    tokenStore.set(session);
    return session;
}

/** Comprueba la sesión guardada contra el backend (p. ej. si rotaron el secreto). */
export async function verifySession(): Promise<boolean> {
    if (!tokenStore.accessToken()) return false;
    try {
        await httpClient.get<{ principal: string }>(apiPath("api", "auth", "me"));
        return true;
    } catch {
        return false; // el 401 ya limpió el token en httpClient
    }
}

/** Solo borra el token local: no hay endpoint de logout. */
export function logout() {
    tokenStore.clear();
}

function tooManyAttemptsMessage(retryAfterSeconds: number | null): string {
    if (retryAfterSeconds === null) return "Demasiados intentos. Intente más tarde.";
    const minutes = Math.max(1, Math.ceil(retryAfterSeconds / 60));
    return `Demasiados intentos. Intente de nuevo en ${minutes} ${minutes === 1 ? "minuto" : "minutos"}.`;
}

/** Mensaje de la pantalla de login para cada respuesta del contrato (400/401/429/503). */
export function loginErrorMessage(error: unknown): string {
    if (isApiError(error)) {
        switch (error.status) {
            case 400:
                return "Complete usuario y contraseña.";
            case 401:
                return "Usuario o contraseña incorrectos.";
            case 429:
                return tooManyAttemptsMessage(error.retryAfterSeconds);
            case 503:
                return "El inicio de sesión no está disponible. Contacte al administrador.";
        }
    }
    return userErrorMessage(error, LOGIN_ERROR_MESSAGE);
}
