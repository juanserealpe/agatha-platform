import { Loader2, Lock } from "lucide-react";
import type { Classification } from "@/services/alertManagement";
import { Button } from "@/components/ui/button";
import {
    AlertDialog,
    AlertDialogClose,
    AlertDialogContent,
    AlertDialogDescription,
    AlertDialogFooter,
    AlertDialogHeader,
    AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { CLASSIFICATION_LABEL } from "../../config/alertManagementDisplay";

interface ConfirmClassificationDialogProps {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    classification: Classification;
    pending: boolean;
    onConfirm: () => void;
}

/**
 * Ventana de confirmación encima del modal de la alerta: la clasificación es irreversible (HU-2.2) y
 * así se confirma sin desplazarse hasta el final del modal.
 */
export function ConfirmClassificationDialog({
    open,
    onOpenChange,
    classification,
    pending,
    onConfirm,
}: ConfirmClassificationDialogProps) {
    return (
        <AlertDialog open={open} onOpenChange={onOpenChange}>
            <AlertDialogContent>
                <AlertDialogHeader>
                    <AlertDialogTitle className="flex items-center gap-2">
                        <Lock className="size-4" aria-hidden />
                        Confirmar clasificación
                    </AlertDialogTitle>
                    <AlertDialogDescription className="text-gray-700">
                        ¿Confirma que la alerta es <strong>{CLASSIFICATION_LABEL[classification]}</strong>? Después no se
                        podrá modificar.
                    </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                    <AlertDialogClose render={<Button type="button" variant="outline" disabled={pending} />}>
                        Volver
                    </AlertDialogClose>
                    <Button type="button" onClick={onConfirm} disabled={pending}>
                        {pending && <Loader2 className="animate-spin" aria-hidden />}
                        {pending ? "Guardando…" : "Sí, clasificar"}
                    </Button>
                </AlertDialogFooter>
            </AlertDialogContent>
        </AlertDialog>
    );
}
