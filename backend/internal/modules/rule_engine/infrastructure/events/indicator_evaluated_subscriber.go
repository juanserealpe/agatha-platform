package events

import (
	"context"

	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// IndicatorEvaluatedSubscriber recibe cada ventana evaluada por Processing.
// Por ahora solo la registra en el log; acá se conectarán las reglas (p. ej.
// el umbral del indicador).
type IndicatorEvaluatedSubscriber struct {
	logger *logger.Logger
}

// NewIndicatorEvaluatedSubscriber crea el subscriber.
func NewIndicatorEvaluatedSubscriber(logger *logger.Logger) *IndicatorEvaluatedSubscriber {
	return &IndicatorEvaluatedSubscriber{logger: logger}
}

// Handle registra el indicador recibido. Idempotente: no tiene efectos.
func (s *IndicatorEvaluatedSubscriber) Handle(_ context.Context, notice IndicatorEvaluatedNotice) {
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

	s.logger.Info("evento de indicador evaluado recibido", fields...)
}
