package services

import (
	"context"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/repositories"
	domainServices "github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// DefaultObservedDays es el período de GET …/observed sin ?days.
const DefaultObservedDays = 90

// MotionObservedResult es lo observado en los últimos Days días del punto.
type MotionObservedResult struct {
	PointID   int64
	Days      int
	From, To  time.Time
	VersionID int64
	Rows      []domainServices.MotionObservedRow
}

// MotionObservedService (§8 de motion-v2, P2, solo lectura) mide en
// motion_evaluations, por cada fila de la tabla vigente y cada eje, p95, p99
// y máximo del cambio en esa ventana fuera de los episodios: la base para que
// TGI defina lo normal de cada dispositivo.
type MotionObservedService struct {
	thresholds  repositories.MotionThresholdRepository
	evaluations repositories.MotionEvaluationRepository
	clock       ports.Clock
	log         *logger.Logger
}

func NewMotionObservedService(thresholds repositories.MotionThresholdRepository, evaluations repositories.MotionEvaluationRepository,
	clock ports.Clock, log *logger.Logger) *MotionObservedService {
	return &MotionObservedService{thresholds: thresholds, evaluations: evaluations, clock: clock, log: log}
}

// Observed mide los últimos days días hasta la última hora evaluada del punto.
func (s *MotionObservedService) Observed(ctx context.Context, pointID int64, days int) (MotionObservedResult, error) {
	if pointID <= 0 {
		return MotionObservedResult{}, domain.ErrInvalidMeasuringPointID
	}
	if days < 1 || days > 365 {
		return MotionObservedResult{}, domain.ErrMotionObservedDays
	}
	current, err := s.thresholds.GetEffectiveAt(ctx, pointID, s.clock.Now().UTC())
	if err != nil {
		s.log.Error("[MotionObservedService] Error consultando la versión vigente", "measuring_point_id", pointID, "error", err)
		return MotionObservedResult{}, domain.ErrConsultingThresholds
	}
	if current == nil {
		return MotionObservedResult{}, domain.ErrMotionThresholdsNotFound
	}
	last, err := s.evaluations.GetLastBefore(ctx, pointID, time.Time{})
	if err != nil {
		s.log.Error("[MotionObservedService] Error consultando la última hora", "measuring_point_id", pointID, "error", err)
		return MotionObservedResult{}, domain.ErrConsultingMotionEvaluations
	}
	res := MotionObservedResult{PointID: pointID, Days: days, VersionID: current.ID}
	if last == nil {
		return res, nil
	}
	res.To = last.EvaluatedAt.UTC()
	res.From = res.To.Add(-time.Duration(days) * 24 * time.Hour)
	rows, err := s.evaluations.ListRange(ctx, pointID, res.From.Add(-entities.MotionRuleMaxHours*time.Hour), res.To)
	if err != nil {
		s.log.Error("[MotionObservedService] Error leyendo motion_evaluations", "measuring_point_id", pointID, "error", err)
		return MotionObservedResult{}, domain.ErrConsultingMotionEvaluations
	}
	res.Rows = domainServices.MotionObserved(rows, current.Params.Rules, res.From, res.To)
	return res, nil
}
