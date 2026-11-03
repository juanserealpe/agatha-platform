// Package services son los casos de uso del módulo de notificaciones.
package services

import (
	"context"
	"strconv"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// AlertPushService envía cada aviso de alerta como push al topic de FCM al que
// se suscriben las apps móviles.
type AlertPushService struct {
	sender ports.PushSender
	topic  string
	log    *logger.Logger
}

func NewAlertPushService(sender ports.PushSender, topic string, log *logger.Logger) *AlertPushService {
	return &AlertPushService{sender: sender, topic: topic, log: log}
}

// Notify envía el aviso. Un aviso atrasado (Late) no se envía: sent = false
// sin error. Un error del sender se devuelve sin reintentar.
func (s *AlertPushService) Notify(ctx context.Context, n entities.AlertNotice) (sent bool, err error) {
	if n.Late {
		s.log.Info("[AlertPushService] Aviso atrasado: no se envía push",
			"message_id", n.MessageID, "alert_code", n.AlertCode, "evaluated_at", n.EvaluatedAt)
		return false, nil
	}

	providerID, err := s.sender.Send(ctx, toPush(s.topic, n))
	if err != nil {
		return false, err
	}
	s.log.Info("[AlertPushService] Push enviado",
		"message_id", n.MessageID, "alert_code", n.AlertCode, "topic", s.topic, "provider_id", providerID)
	return true, nil
}

func toPush(topic string, n entities.AlertNotice) entities.Push {
	return entities.Push{
		Topic: topic,
		Title: n.Title,
		Body:  n.Body,
		Data: map[string]string{
			"message_id":         strconv.FormatInt(n.MessageID, 10),
			"alert_id":           strconv.FormatInt(n.AlertID, 10),
			"alert_code":         n.AlertCode,
			"measuring_point_id": strconv.FormatInt(n.MeasuringPointID, 10),
			"event_type":         n.EventType,
			"evaluated_at":       n.EvaluatedAt.UTC().Format(time.RFC3339),
			"code":               n.Code,
			"level":              n.Level,
			"max_level":          n.MaxLevel,
			"alert_state":        n.AlertState,
			"source":             n.Source,
		},
	}
}
