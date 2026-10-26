// Package events recibe los eventos que le interesan a Alert Management. No
// conoce el bus ni al Rule Engine: internal/bootstrap traduce y suscribe.
package events

import (
	"context"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// LevelProcessor es el caso de uso que procesa cada hora (en producción, *services.ProcessingService).
type LevelProcessor interface {
	Process(ctx context.Context, notice entities.LevelNotice, source entities.Source) (services.ProcessResult, error)
}

// LevelEvaluatedSubscriber recibe cada hora de rule_engine.level_evaluated.
type LevelEvaluatedSubscriber struct {
	processor LevelProcessor
	log       *logger.Logger
}

func NewLevelEvaluatedSubscriber(processor LevelProcessor, log *logger.Logger) *LevelEvaluatedSubscriber {
	return &LevelEvaluatedSubscriber{processor: processor, log: log}
}

// Handle procesa la hora en vivo. Un error se registra y no se reintenta: el
// evento es una pista; el catch-up del minuto 55 completa la hora desde rule_evaluations.
func (s *LevelEvaluatedSubscriber) Handle(ctx context.Context, notice entities.LevelNotice) {
	r, err := s.processor.Process(ctx, notice, entities.SourceLive)
	if err != nil {
		s.log.Error("[LevelEvaluatedSubscriber] No se pudo procesar la hora (la completará el catch-up)",
			"measuring_point_id", notice.MeasuringPointID, "evaluated_at", notice.EvaluatedAt, "status", notice.Status, "error", err)
		return
	}
	s.log.Debug("[LevelEvaluatedSubscriber] Hora procesada",
		"measuring_point_id", notice.MeasuringPointID, "evaluated_at", notice.EvaluatedAt,
		"outcome", r.Outcome, "messages", r.Messages, "notifications", r.Notifications)
}
