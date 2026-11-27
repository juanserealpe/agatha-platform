import { useId, useState, type FormEvent } from "react";
import { Navigate, useLocation, useNavigate } from "react-router";
import { useMutation } from "@tanstack/react-query";
import tgiLogo from "@/assets/tgi-logo.png";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { redirectAfterLogin } from "@/app/sessionRedirect";
import { login, loginErrorMessage, tokenStore } from "@/services/auth";

const EMPTY_FIELDS_MESSAGE = "Complete usuario y contraseña.";

interface Credentials {
    username: string;
    password: string;
}

export function LoginPage() {
    const navigate = useNavigate();
    const location = useLocation();
    const usernameId = useId();
    const passwordId = useId();
    const errorId = useId();
    const [username, setUsername] = useState("");
    const [password, setPassword] = useState("");
    const [error, setError] = useState<string | null>(null);
    // A dónde volver: la página que pidió el login, o monitoreo.
    const target = redirectAfterLogin(location.state);

    const mutation = useMutation({
        mutationFn: ({ username, password }: Credentials) => login(username, password),
        onSuccess: () => navigate(target, { replace: true }),
        onError: (failure) => setError(loginErrorMessage(failure)),
    });

    // Ya hay sesión (o acaba de iniciarse): directo a la app, al mismo destino que tras el login.
    if (tokenStore.get()) return <Navigate to={target} replace />;

    function handleSubmit(event: FormEvent<HTMLFormElement>) {
        event.preventDefault();
        const trimmed = username.trim();
        if (trimmed === "" || password === "") {
            setError(EMPTY_FIELDS_MESSAGE);
            return;
        }
        setError(null);
        mutation.mutate({ username: trimmed, password });
    }

    return (
        <div className="flex min-h-screen items-center justify-center bg-tgi-fondo p-4">
            <form
                onSubmit={handleSubmit}
                noValidate
                aria-describedby={error ? errorId : undefined}
                className="flex w-full max-w-sm flex-col gap-5 rounded-2xl bg-white p-6 shadow-sm"
            >
                <div className="flex flex-col items-center gap-3 text-center">
                    <img src={tgiLogo} alt="TGI · Grupo Energía Bogotá" className="h-11 w-auto" />
                    <div>
                        <h1 className="text-2xl font-bold tracking-wide text-gray-900">AGATHA</h1>
                        <p className="mt-1 text-sm text-gray-600">Inicie sesión para continuar.</p>
                    </div>
                </div>

                <div className="flex flex-col gap-2">
                    <Label htmlFor={usernameId}>Usuario</Label>
                    <Input
                        id={usernameId}
                        name="username"
                        autoComplete="username"
                        autoFocus
                        value={username}
                        onChange={(event) => setUsername(event.target.value)}
                        aria-invalid={error ? true : undefined}
                    />
                </div>

                <div className="flex flex-col gap-2">
                    <Label htmlFor={passwordId}>Contraseña</Label>
                    <Input
                        id={passwordId}
                        name="password"
                        type="password"
                        autoComplete="current-password"
                        value={password}
                        onChange={(event) => setPassword(event.target.value)}
                        aria-invalid={error ? true : undefined}
                    />
                </div>

                {error && (
                    <p id={errorId} role="alert" className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">
                        {error}
                    </p>
                )}

                <Button type="submit" size="lg" disabled={mutation.isPending}>
                    {mutation.isPending ? "Ingresando..." : "Iniciar sesión"}
                </Button>
            </form>
        </div>
    );
}
