package services

import (
	"context"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/contract"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/domain/entities"
)

// publishIndicatorEvaluated publica, post-commit, un evento por ventana
// evaluada (24 h y 6 h de cada hora). Solo si el lote se insertó completo:
// SaveAll devuelve cuántas filas insertó pero no cuáles, y con un conflicto
// parcial (otro proceso, con su propio bus, ya insertó parte) un evento podría
// describir una fila distinta de la persistida. Los consumidores reconcilian
// contra feature_evaluations. Un fallo al publicar nunca falla la evaluación.
func (s *EvaluationService) publishIndicatorEvaluated(
	ctx context.Context,
	pointID int64,
	evals []entities.FeatureEvaluation,
	inserted int,
) {
	if s.publisher == nil || inserted == 0 {
		return
	}

	if inserted < len(evals) {
		s.log.Warn("[EvaluationService] Lote insertado en parte: no se publican los eventos de indicador evaluado",
			"measuring_point_id", pointID, "rows", len(evals), "inserted", inserted)
		return
	}

	occurredAt := s.clock.Now().UTC()

	for _, e := range evals {
		if err := s.publisher.PublishIndicatorEvaluated(ctx, toIndicatorEvaluated(e, occurredAt)); err != nil {
			s.log.Warn("[EvaluationService] No se pudo publicar el evento de indicador evaluado",
				"measuring_point_id", pointID,
				"window_hours", e.WindowHours,
				"evaluated_at", e.EvaluatedAt,
				"error", err,
			)
		}
	}
}

func toIndicatorEvaluated(e entities.FeatureEvaluation, occurredAt time.Time) contract.IndicatorEvaluated {
	return contract.IndicatorEvaluated{
		MeasuringPointID: e.MeasuringPointID,
		WindowHours:      e.WindowHours,
		EvaluatedAt:      e.EvaluatedAt.UTC(),
		Status:           string(e.Status),
		NoCalcReason:     string(e.NoCalcReason),
		I:                e.I,
		LowConfidence:    e.LowConfidence,
		Trigger:          string(e.Trigger),
		OccurredAt:       occurredAt,
		SchemaVersion:    contract.SchemaVersion,
	}
}
