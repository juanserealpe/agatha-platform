import { describe, expect, it } from "vitest";
import { toAlertId } from "@/services/alertManagement";
import type { AlertNotificationEvent } from "@/services/realtime";
import {
    buildBrowserNotification,
    setBrowserNotificationTitlePrefix,
    shouldShowBrowserNotification,
} from "./browserNotification";

const notice = (overrides: Partial<AlertNotificationEvent> = {}): AlertNotificationEvent => ({
    message_id: 1,
    alert_id: toAlertId(142),
    alert_code: "ALR-000142",
    measuring_point_id: 11556,
    event_type: "FUGA",
    code: "UP_ROJO",
    level: "ROJO",
    max_level: "ROJO",
    classification: null,
    title: "La alerta ALR-000142 subió a Rojo",
    body: "El índice de ruido pasó de 0,30 a 0,41; Rojo empieza en 0,35.",
    late: false,
    started_at: "2026-10-30T12:00:00Z",
    evaluated_at: "2026-10-30T15:00:00Z",
    occurred_at: "2026-10-30T15:00:05Z",
    ...overrides,
});

describe("shouldShowBrowserNotification", () => {
    it("solo con permiso y la pestaña oculta", () => {
        expect(shouldShowBrowserNotification("granted", true)).toBe(true);
        expect(shouldShowBrowserNotification("granted", false)).toBe(false);
        expect(shouldShowBrowserNotification("default", true)).toBe(false);
        expect(shouldShowBrowserNotification("denied", true)).toBe(false);
        expect(shouldShowBrowserNotification("unsupported", true)).toBe(false);
    });
});

describe("buildBrowserNotification", () => {
    it("usa el texto del backend y agrupa por alerta con el tag", () => {
        const { title, options } = buildBrowserNotification(notice());
        expect(title).toBe("La alerta ALR-000142 subió a Rojo");
        expect(options).toMatchObject({ body: notice().body, tag: "ALR-000142", requireInteraction: true });
    });

    it("marca el aviso atrasado y deja que el sistema cierre los que no son rojos", () => {
        const { title, options } = buildBrowserNotification(notice({ late: true, level: "NARANJA" }));
        expect(title).toBe("La alerta ALR-000142 subió a Rojo (Atrasado)");
        expect(options.requireInteraction).toBe(false);
    });

    it("antepone el prefijo configurado (demo)", () => {
        setBrowserNotificationTitlePrefix("[SIMULACIÓN] ");
        try {
            expect(buildBrowserNotification(notice({ late: true })).title).toBe(
                "[SIMULACIÓN] La alerta ALR-000142 subió a Rojo (Atrasado)"
            );
        } finally {
            setBrowserNotificationTitlePrefix("");
        }
    });
});
