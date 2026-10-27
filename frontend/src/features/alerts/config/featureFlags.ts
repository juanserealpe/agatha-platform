/**
 * Evidencias fotográficas de una alerta: quedan para después del MVP (no hay endpoint todavía).
 * Con el flag apagado la sección no se renderiza ni hace peticiones. Se enciende con
 * VITE_FEATURE_ALERT_EVIDENCE=true cuando exista el endpoint.
 */
export const ALERT_EVIDENCE_ENABLED = import.meta.env.VITE_FEATURE_ALERT_EVIDENCE === "true";
