// Package events recibe los eventos de Alert Management que le interesan a
// Notifications. No conoce el bus ni a Alert Management: internal/bootstrap
// traduce los contratos a AlertNotice/AlertChange y suscribe.
package events

import (
	"context"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// NoticeDispatcher es el caso de uso de un canal (en producción, *services.NoticeDispatcher).
type NoticeDispatcher interface {
	Dispatch(ctx context.Context, n entities.AlertNotice)
}

// AlertNotificationSubscriber recibe cada aviso de alert_management.alert_notification
// para un canal. Hay uno por canal, cada uno con su suscripción.
type AlertNotificationSubscriber struct {
	dispatcher NoticeDispatcher
}

func NewAlertNotificationSubscriber(dispatcher NoticeDispatcher) *AlertNotificationSubscriber {
	return &AlertNotificationSubscriber{dispatcher: dispatcher}
}

// Handle entrega el aviso al caso de uso del canal.
func (s *AlertNotificationSubscriber) Handle(ctx context.Context, n entities.AlertNotice) {
	s.dispatcher.Dispatch(ctx, n)
}

// AlertChangedSubscriber recibe cada foto de alert_management.alert_changed y
// la entrega al canal web (el único que la usa).
type AlertChangedSubscriber struct {
	channel ports.ChangeChannel
	log     *logger.Logger
}

func NewAlertChangedSubscriber(channel ports.ChangeChannel, log *logger.Logger) *AlertChangedSubscriber {
	return &AlertChangedSubscriber{channel: channel, log: log}
}

// Handle entrega la foto. Un error se registra y sigue: el frontend se
// resincroniza por REST al reconectar.
func (s *AlertChangedSubscriber) Handle(ctx context.Context, c entities.AlertChange) {
	if err := s.channel.DeliverChange(ctx, c); err != nil {
		s.log.Error("[AlertChangedSubscriber] No se pudo entregar el cambio de la alerta",
			"alert", c.AlertCode, "change", c.Change, "error", err)
	}
}
