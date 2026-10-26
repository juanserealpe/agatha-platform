package services

import (
	"context"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/repositories"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// endOfTime acota "la última fila del punto" (GetPrevious con before = nunca).
var endOfTime = time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)

// PointCatchUp es el resumen de un punto en una corrida del catch-up.
type PointCatchUp struct {
	PointID int64
	// WithoutHistory: el punto no tiene ninguna fila en rule_evaluations; se
	// saltea (su histórico lo carga rule_engine -backfill, que no publica).
	WithoutHistory bool
	Hours          int // horas CALCULATED de 24 h posteriores a la última fila
	Saved          int
	Published      int
	Err            error
}

// CatchUpService es la red de seguridad de la evaluación en vivo (como el
// reconciliador de Processing): interpreta, CON publicación, las horas
// CALCULATED de 24 h de feature_evaluations posteriores a la última fila de
// cada punto. Cubre las horas que Processing guardó sin que llegara su evento:
// lote insertado en parte (no publica), evento perdido por el bus o error de
// BD al consumirlo.
//
// Usa EXACTAMENTE EvaluateIndicator (mismo lock por punto que el subscriber):
// si el evento llega después, la hora es DUPLICATE y no se republica. Solo
// avanza hacia adelante; un hueco en medio del histórico se arregla con
// -rebuild. Las NO_CALCULABLE no se reenvían (no tienen fila que leer).
type CatchUpService struct {
	evaluator   *EvaluationService
	source      ports.IndicatorSource
	evaluations repositories.RuleEvaluationRepository
	batchSize   int
	log         *logger.Logger
}

func NewCatchUpService(
	evaluator *EvaluationService,
	source ports.IndicatorSource,
	evaluations repositories.RuleEvaluationRepository,
	batchSize int,
	log *logger.Logger,
) *CatchUpService {
	if batchSize <= 0 {
		batchSize = defaultBackfillBatchSize
	}
	return &CatchUpService{evaluator: evaluator, source: source, evaluations: evaluations, batchSize: batchSize, log: log}
}

// CatchUp corre una pasada por todos los puntos de feature_evaluations. Un
// error en un punto detiene ese punto (el orden importa) y no a los demás;
// solo falla si no se pueden listar los puntos.
func (s *CatchUpService) CatchUp(ctx context.Context) ([]PointCatchUp, error) {
	points, err := s.source.ListPoints(ctx, time.Time{}, time.Time{})
	if err != nil {
		s.log.Error("[CatchUpService] Error listando los puntos de feature_evaluations", "error", err)
		return nil, domain.ErrConsultingIndicators
	}

	summary := make([]PointCatchUp, 0, len(points))
	for _, pointID := range points {
		if ctx.Err() != nil {
			s.log.Warn("[CatchUpService] Catch-up interrumpido", "error", ctx.Err())
			break
		}
		summary = append(summary, s.catchUpPoint(ctx, pointID))
	}
	return summary, nil
}

func (s *CatchUpService) catchUpPoint(ctx context.Context, pointID int64) PointCatchUp {
	item := PointCatchUp{PointID: pointID}

	last, err := s.evaluations.GetPrevious(ctx, pointID, entities.EventTypeLeak, endOfTime, 1)
	if err != nil {
		s.log.Error("[CatchUpService] Error consultando la última hora interpretada",
			"measuring_point_id", pointID, "error", err)
		item.Err = domain.ErrConsultingRuleEvaluations
		return item
	}
	if len(last) == 0 {
		item.WithoutHistory = true
		return item
	}
	after := last[0].EvaluatedAt

	for {
		batch, err := s.source.ListCalculated(ctx, pointID, time.Time{}, time.Time{}, after, s.batchSize)
		if err != nil {
			s.log.Error("[CatchUpService] Error leyendo feature_evaluations",
				"measuring_point_id", pointID, "after", after, "error", err)
			item.Err = domain.ErrConsultingIndicators
			return item
		}

		for _, h := range batch {
			i := h.I
			r, err := s.evaluator.EvaluateIndicator(ctx, IndicatorEvaluatedNotice{
				MeasuringPointID: pointID,
				WindowHours:      WindowHours24,
				EvaluatedAt:      h.EvaluatedAt,
				Status:           StatusCalculated,
				I:                &i,
				DPl:              h.DPl,
				DPlLoaded:        true,
				LowConfidence:    h.LowConfidence,
				Trigger:          TriggerCatchUp,
			})
			if err != nil {
				item.Err = err
				return item
			}
			item.Hours++
			if r.Outcome == OutcomeSaved {
				item.Saved++
			}
			if r.Published {
				item.Published++
			}
		}

		if len(batch) > 0 {
			after = batch[len(batch)-1].EvaluatedAt
		}
		if len(batch) < s.batchSize {
			if item.Hours > 0 {
				s.log.Warn("[CatchUpService] Horas interpretadas sin evento de Processing (evento perdido o no publicado)",
					"measuring_point_id", pointID, "hours", item.Hours, "saved", item.Saved,
					"published", item.Published, "until", after)
			}
			return item
		}
	}
}
