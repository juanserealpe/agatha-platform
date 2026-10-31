import { describe, expect, it } from "vitest";
import { toAlertId } from "@/services/alertManagement";
import type { AlertNotificationEvent } from "@/services/realtime";
import { dismissNotice, ingestNotification, INITIAL_NEW_ALERTS_STATE, sortNotices } from "./newAlerts";

const notice = (messageId: number, overrides: Partial<AlertNotificationEvent> = {}): AlertNotificationEvent => ({
    message_id: messageId,
    alert_id: toAlertId(142),
    alert_code: "ALR-000142",
    measuring_point_id: 11556,
    event_type: "FUGA",
    code: "OPEN",
    level: "AMARILLO",
    max_level: "AMARILLO",
    classification: null,
    title: "Nueva alerta",
    body: "Índice de ruido en 0,18.",
    late: false,
    started_at: "2026-10-30T12:00:00Z",
    evaluated_at: "2026-10-30T14:00:00Z",
    occurred_at: "2026-10-30T14:00:05Z",
    ...overrides,
});

const ids = (notices: AlertNotificationEvent[]) => notices.map((n) => n.message_id);

describe("ingestNotification", () => {
    it("una subida de nivel de la misma alerta reemplaza al aviso anterior", () => {
        const opened = ingestNotification(INITIAL_NEW_ALERTS_STATE, notice(1));
        const raised = ingestNotification(opened, notice(2, { code: "UP_ROJO", level: "ROJO" }));
        expect(ids(raised.notices)).toEqual([2]);
    });

    it("avisos de alertas distintas se acumulan", () => {
        const first = ingestNotification(INITIAL_NEW_ALERTS_STATE, notice(1));
        const second = ingestNotification(first, notice(2, { alert_id: toAlertId(150), alert_code: "ALR-000150" }));
        expect(ids(second.notices)).toEqual([1, 2]);
    });

    it("un aviso reenviado tras una reconexión no se repite, aunque se haya descartado", () => {
        const once = ingestNotification(INITIAL_NEW_ALERTS_STATE, notice(1));
        expect(ingestNotification(once, notice(1))).toBe(once);

        const dismissed = dismissNotice(once, "ALR-000142");
        expect(ingestNotification(dismissed, notice(1)).notices).toEqual([]);
    });

    it("tras descartar, un aviso nuevo de la misma alerta vuelve a mostrarse", () => {
        const dismissed = dismissNotice(ingestNotification(INITIAL_NEW_ALERTS_STATE, notice(1)), "ALR-000142");
        expect(ids(ingestNotification(dismissed, notice(2, { code: "UP_ROJO" })).notices)).toEqual([2]);
    });
});

describe("sortNotices", () => {
    it("mayor nivel primero y, entre iguales, el más reciente", () => {
        const yellow = notice(1, { alert_code: "A", occurred_at: "2026-10-30T16:00:00Z" });
        const redOld = notice(2, { alert_code: "B", level: "ROJO", occurred_at: "2026-10-30T14:00:00Z" });
        const redNew = notice(3, { alert_code: "C", level: "ROJO", occurred_at: "2026-10-30T15:00:00Z" });
        expect(ids(sortNotices([yellow, redOld, redNew]))).toEqual([3, 2, 1]);
    });
});
