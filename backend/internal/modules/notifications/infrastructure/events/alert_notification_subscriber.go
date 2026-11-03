// Package events recibe los eventos que le interesan al módulo de
// notificaciones. No conoce el bus ni a Alert Management: internal/bootstrap
// traduce y suscribe.
package events

import (
	"context"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// AlertNotifier es el caso de uso que envía cada aviso (en producción, *services.AlertPushService).
type AlertNotifier interface {
	Notify(ctx context.Context, n entities.AlertNotice) (sent bool, err error)
}

// AlertNotificationSubscriber recibe cada aviso de alert_management.alert_notification.
type AlertNotificationSubscriber struct {
	notifier AlertNotifier
	log      *logger.Logger
}

func NewAlertNotificationSubscriber(notifier AlertNotifier, log *logger.Logger) *AlertNotificationSubscriber {
	return &AlertNotificationSubscriber{notifier: notifier, log: log}
}

// Handle envía el aviso. Un error se registra y no se reintenta: el evento es
// best-effort; la fuente de verdad sigue siendo alert_messages (NOTIFICACION).
func (s *AlertNotificationSubscriber) Handle(ctx context.Context, n entities.AlertNotice) {
	if _, err := s.notifier.Notify(ctx, n); err != nil {
		s.log.Error("[AlertNotificationSubscriber] No se pudo enviar el push",
			"message_id", n.MessageID, "alert_code", n.AlertCode, "error", err)
	}
}
