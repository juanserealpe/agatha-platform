import { LogOut } from "lucide-react";
import { useQueryClient } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { logout } from "@/services/auth";

/**
 * Cierra la sesión local (no hay endpoint de logout). Al borrar el token, RequireSession lleva al login y
 * desmonta la app, lo que cierra el stream; la caché se vacía para no dejar datos del admin.
 */
export function LogoutButton() {
    const queryClient = useQueryClient();

    function handleLogout() {
        // Primero el token: así ninguna consulta vuelve a pedirse con la caché recién vaciada.
        logout();
        queryClient.clear();
    }

    return (
        <Button variant="outline" size="sm" onClick={handleLogout}>
            <LogOut aria-hidden />
            Cerrar sesión
        </Button>
    );
}
