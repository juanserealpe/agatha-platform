import { createContext, useContext, type ReactNode } from "react";

/**
 * Lo que el layout necesita saber del modo (ver appMode.ts). El overlay de la demo llega ya construido desde su
 * chunk: así ningún texto ni componente de la demo entra en el chunk principal.
 */
export interface AppModeValue {
    demo: boolean;
    /** Se dibuja en RootLayout, en todas las páginas; null en la app real. */
    overlay: ReactNode;
}

export const REAL_APP_MODE: AppModeValue = { demo: false, overlay: null };

export const AppModeContext = createContext<AppModeValue>(REAL_APP_MODE);

export function useAppMode(): AppModeValue {
    return useContext(AppModeContext);
}
