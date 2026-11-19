package events

import (
	"context"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// MotionProcessor es el caso de uso que procesa cada hora de movimiento.
type MotionProcessor interface {
	ProcessMotion(ctx context.Context, notice entities.MotionNotice, source entities.Source) (services.ProcessResult, error)
}

// MotionEvaluatedSubscriber recibe cada hora de rule_engine.motion_evaluated.
type MotionEvaluatedSubscriber struct {
	processor MotionProcessor
	log       *logger.Logger
}

func NewMotionEvaluatedSubscriber(processor MotionProcessor, log *logger.Logger) *MotionEvaluatedSubscriber {
	return &MotionEvaluatedSubscriber{processor: processor, log: log}
}

// Handle procesa la hora en vivo. Un error se registra: el catch-up del
// minuto 55 completa la hora desde motion_evaluations.
func (s *MotionEvaluatedSubscriber) Handle(ctx context.Context, notice entities.MotionNotice) {
	r, err := s.processor.ProcessMotion(ctx, notice, entities.SourceLive)
	if err != nil {
		s.log.Error("[MotionEvaluatedSubscriber] No se pudo procesar la hora de movimiento (la completará el catch-up)",
			"measuring_point_id", notice.MeasuringPointID, "evaluated_at", notice.EvaluatedAt, "status", notice.Status, "error", err)
		return
	}
	s.log.Debug("[MotionEvaluatedSubscriber] Hora de movimiento procesada",
		"measuring_point_id", notice.MeasuringPointID, "evaluated_at", notice.EvaluatedAt,
		"outcome", r.Outcome, "messages", r.Messages, "notifications", r.Notifications)
}
