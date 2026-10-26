// Package events recibe los eventos que le interesan al Rule Engine. No conoce
// el bus ni a los publicadores: internal/bootstrap traduce y suscribe.
package events

import (
	"context"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// IndicatorEvaluator es el caso de uso que interpreta cada ventana evaluada
// (en producción, *services.EvaluationService).
type IndicatorEvaluator interface {
	EvaluateIndicator(ctx context.Context, notice services.IndicatorEvaluatedNotice) (services.EvaluationResult, error)
}

// IndicatorEvaluatedSubscriber recibe cada ventana evaluada por Processing y
// la entrega a EvaluateIndicator (que filtra 24 h CALCULATED, guarda y publica).
type IndicatorEvaluatedSubscriber struct {
	evaluator IndicatorEvaluator
	logger    *logger.Logger
}

// NewIndicatorEvaluatedSubscriber crea el subscriber.
func NewIndicatorEvaluatedSubscriber(evaluator IndicatorEvaluator, logger *logger.Logger) *IndicatorEvaluatedSubscriber {
	return &IndicatorEvaluatedSubscriber{evaluator: evaluator, logger: logger}
}

// Handle interpreta el indicador recibido. Un error se registra y no se
// reintenta: el evento es una pista y feature_evaluations la fuente de verdad
// (el catch-up horario completa la hora perdida).
func (s *IndicatorEvaluatedSubscriber) Handle(ctx context.Context, notice services.IndicatorEvaluatedNotice) {
	fields := []interface{}{
		"measuring_point_id", notice.MeasuringPointID,
		"window_hours", notice.WindowHours,
		"evaluated_at", notice.EvaluatedAt,
		"status", notice.Status,
	}
	if notice.I != nil {
		fields = append(fields, "i", *notice.I)
	}
	if notice.NoCalcReason != "" {
		fields = append(fields, "no_calc_reason", notice.NoCalcReason)
	}
	fields = append(fields,
		"low_confidence", notice.LowConfidence,
		"trigger", notice.Trigger,
	)

	s.logger.Debug("evento de indicador evaluado recibido", fields...)

	if _, err := s.evaluator.EvaluateIndicator(ctx, notice); err != nil {
		s.logger.Error("no fue posible interpretar el indicador evaluado", append(fields, "error", err)...)
	}
}
