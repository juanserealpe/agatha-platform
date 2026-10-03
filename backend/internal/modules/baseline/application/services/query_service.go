package services

import (
	"context"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/repositories"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// QueryService implementa UC-B3: la línea base vigente de un punto en un instante.
type QueryService struct {
	history   repositories.HistoryReader
	baselines repositories.BaselineRepository
	log       *logger.Logger
}

func NewQueryService(
	history repositories.HistoryReader,
	baselines repositories.BaselineRepository,
	log *logger.Logger,
) *QueryService {
	return &QueryService{history: history, baselines: baselines, log: log}
}

// GetEffective devuelve la versión con valid_from <= at < valid_to y el primer
// registro de B2. Ausencia de versión NO es error (Version = nil).
func (s *QueryService) GetEffective(ctx context.Context, pointID int64, at time.Time,
) (EffectiveResult, error) {
	if pointID <= 0 {
		s.log.Error("[QueryService] Punto de medición inválido", "measuring_point_id", pointID)
		return EffectiveResult{}, domain.ErrInvalidMeasuringPointID
	}

	version, err := s.baselines.GetEffective(ctx, pointID, at)
	if err != nil {
		s.log.Error("[QueryService] Error consultando la línea base vigente",
			"measuring_point_id", pointID, "at", at.UTC(), "error", err)
		return EffectiveResult{}, domain.ErrConsultingBaseline
	}

	firstSeen, err := s.history.GetFirstSeenAt(ctx, pointID)
	if err != nil {
		s.log.Error("[QueryService] Error consultando el primer registro del dispositivo",
			"measuring_point_id", pointID, "error", err)
		return EffectiveResult{}, domain.ErrConsultingHistory
	}

	return EffectiveResult{Version: version, FirstSeenAt: firstSeen}, nil
}
