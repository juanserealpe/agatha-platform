package realtime

import (
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/notifications/domain/entities"
)

// noticePayload es el data de alert.notification (banner, sonido y
// notificación del navegador). El frontend deduplica por message_id.
type noticePayload struct {
	MessageID        int64     `json:"message_id"`
	AlertID          int64     `json:"alert_id"`
	AlertCode        string    `json:"alert_code"`
	MeasuringPointID int64     `json:"measuring_point_id"`
	EventType        string    `json:"event_type"`
	Code             string    `json:"code"`
	Level            string    `json:"level"`
	MaxLevel         string    `json:"max_level"`
	Classification   *string   `json:"classification"`
	Title            string    `json:"title"`
	Body             string    `json:"body"`
	Late             bool      `json:"late"`
	StartedAt        time.Time `json:"started_at"`
	EvaluatedAt      time.Time `json:"evaluated_at"`
	OccurredAt       time.Time `json:"occurred_at"`
}

// changePayload es el data de alert.changed: el estado actual de la alerta
// (mapa y panel). Las reglas para descartar estados viejos están en el
// contrato de Alert Management (AlertChanged).
type changePayload struct {
	AlertID          int64     `json:"alert_id"`
	AlertCode        string    `json:"alert_code"`
	MeasuringPointID int64     `json:"measuring_point_id"`
	EventType        string    `json:"event_type"`
	Change           string    `json:"change"`
	State            string    `json:"state"`
	CurrentLevel     *string   `json:"current_level"`
	MaxLevel         string    `json:"max_level"`
	Blinking         bool      `json:"blinking"`
	ManagementStatus string    `json:"management_status"`
	Classification   *string   `json:"classification"`
	StartedAt        time.Time `json:"started_at"`
	EvaluatedAt      time.Time `json:"evaluated_at"`
	OccurredAt       time.Time `json:"occurred_at"`
}

func toNoticePayload(n entities.AlertNotice) noticePayload {
	return noticePayload{
		MessageID:        n.MessageID,
		AlertID:          n.AlertID,
		AlertCode:        n.AlertCode,
		MeasuringPointID: n.MeasuringPointID,
		EventType:        n.EventType,
		Code:             n.Code,
		Level:            n.Level,
		MaxLevel:         n.MaxLevel,
		Classification:   strOrNil(n.Classification),
		Title:            n.Title,
		Body:             n.Body,
		Late:             n.Late,
		StartedAt:        n.StartedAt,
		EvaluatedAt:      n.EvaluatedAt,
		OccurredAt:       n.OccurredAt,
	}
}

func toChangePayload(c entities.AlertChange) changePayload {
	return changePayload{
		AlertID:          c.AlertID,
		AlertCode:        c.AlertCode,
		MeasuringPointID: c.MeasuringPointID,
		EventType:        c.EventType,
		Change:           c.Change,
		State:            c.State,
		CurrentLevel:     strOrNil(c.CurrentLevel),
		MaxLevel:         c.MaxLevel,
		Blinking:         c.Blinking,
		ManagementStatus: c.ManagementStatus,
		Classification:   strOrNil(c.Classification),
		StartedAt:        c.StartedAt,
		EvaluatedAt:      c.EvaluatedAt,
		OccurredAt:       c.OccurredAt,
	}
}

// strOrNil devuelve null en el JSON cuando el valor no existe, igual que la API
// de alertas (classification sin clasificar, current_level sin nivel).
func strOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
