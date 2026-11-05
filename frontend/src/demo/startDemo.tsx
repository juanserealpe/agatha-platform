import type { ReactNode } from "react";
import { DemoOverlay } from "./DemoOverlay";
import { installDemo } from "./installDemo";

/**
 * Entrada del chunk de la demo (main.tsx lo carga con `import()` solo en `/demo`): instala los servicios
 * simulados y devuelve el overlay que RootLayout dibuja en todas las páginas.
 */
export function startDemo(): ReactNode {
    return <DemoOverlay controller={installDemo()} />;
}
