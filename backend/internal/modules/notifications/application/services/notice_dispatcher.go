// Package services son los casos de uso de Notifications.
package services

import (
	"context"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/policies"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// NoticeDispatcher aplica la política de un canal y le entrega los avisos que
// pasan. Hay uno por canal, cada uno en su propia suscripción del bus: si un
// canal se atrasa, los demás no lo notan.
type NoticeDispatcher struct {
	channel string
	policy  policies.ChannelPolicy
	target  ports.NoticeChannel
	log     *logger.Logger
}

// NewNoticeDispatcher crea el despachador. channel es el nombre del canal (logs).
func NewNoticeDispatcher(channel string, policy policies.ChannelPolicy, target ports.NoticeChannel, log *logger.Logger) *NoticeDispatcher {
	return &NoticeDispatcher{channel: channel, policy: policy, target: target, log: log}
}

// Dispatch evalúa y entrega un aviso. Un evento no tiene a quién devolverle el
// error: se registra y sigue (alert_messages es la fuente de verdad).
func (d *NoticeDispatcher) Dispatch(ctx context.Context, n entities.AlertNotice) {
	if v := d.policy.Evaluate(n); !v.Send {
		d.log.Info("[NoticeDispatcher] Aviso omitido por la política del canal",
			"channel", d.channel, "alert", n.AlertCode, "message_id", n.MessageID, "code", n.Code, "reason", v.Reason)
		return
	}
	if err := d.target.DeliverNotice(ctx, n); err != nil {
		d.log.Error("[NoticeDispatcher] No se pudo entregar el aviso",
			"channel", d.channel, "alert", n.AlertCode, "message_id", n.MessageID, "error", err)
	}
}
