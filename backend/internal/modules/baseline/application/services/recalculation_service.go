package services

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/repositories"
	domainServices "github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/features"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// RecalculationService implementa UC-B1 (Recalculate) y UC-B2 (RecalculateDue).
type RecalculationService struct {
	history   repositories.HistoryReader
	baselines repositories.BaselineRepository
	events    repositories.KnownEventsProvider
	estimator domainServices.RobustBaselineEstimator
	policy    domainServices.RecalculationPolicy
	cfg       domainServices.Config
	log       *logger.Logger
	now       func() time.Time

	// lastSkip evita inundar logs: un punto sin datos suficientes se reintenta
	// cada tick, pero solo se loguea cuando cambia el motivo.
	mu       sync.Mutex
	lastSkip map[int64]SkippedReason
}

func NewRecalculationService(
	history repositories.HistoryReader,
	baselines repositories.BaselineRepository,
	events repositories.KnownEventsProvider,
	cfg domainServices.Config,
	log *logger.Logger,
	now func() time.Time,
) *RecalculationService {
	if now == nil {
		now = time.Now
	}

	return &RecalculationService{
		history:   history,
		baselines: baselines,
		events:    events,
		estimator: domainServices.NewRobustBaselineEstimator(cfg),
		policy:    domainServices.NewRecalculationPolicy(cfg),
		cfg:       cfg,
		log:       log,
		now:       now,
		lastSkip:  map[int64]SkippedReason{},
	}
}

// Recalculate (UC-B1) estima la línea base de un punto con el histórico
// [AsOf − HistoryDays, AsOf), sin los eventos conocidos de ese rango, y la
// versiona (salvo DryRun). Todos los errores
// devueltos son *AppError de baseline/domain.
func (s *RecalculationService) Recalculate(ctx context.Context, cmd RecalculateCommand,
) (RecalculationResult, error) {
	result := RecalculationResult{PointID: cmd.PointID}

	if cmd.PointID <= 0 {
		s.log.Error("[RecalculationService] Punto de medición inválido", "measuring_point_id", cmd.PointID)
		return result, domain.ErrInvalidMeasuringPointID
	}

	asOf := cmd.AsOf.UTC()
	from := asOf.Add(-s.cfg.HistoryWindow())

	raw, err := s.history.GetSamples(ctx, cmd.PointID, from, asOf)
	if err != nil {
		s.log.Error("[RecalculationService] Error consultando el histórico",
			"measuring_point_id", cmd.PointID, "error", err)
		return result, domain.ErrConsultingHistory
	}

	eventsFrom, eventsTo := s.cfg.KnownEventsRange(from, asOf)
	known, err := s.events.GetKnownEvents(ctx, cmd.PointID, eventsFrom, eventsTo)
	if err != nil {
		s.log.Error("[RecalculationService] Error consultando los eventos conocidos",
			"measuring_point_id", cmd.PointID, "error", err)
		return result, domain.ErrConsultingKnownEvents
	}

	samples := features.Normalize(raw)
	estimation := s.estimator.EstimateExcluding(samples, from, asOf, known)
	result.Passes = estimation.Passes

	if estimation.Status == domainServices.EstimationInsufficientHistory {
		result.SkippedReason = SkippedInsufficientHistory
		s.logSkipOnce(cmd.PointID, SkippedInsufficientHistory,
			"[RecalculationService] Histórico insuficiente; no se crea versión",
			"measuring_point_id", cmd.PointID, "samples", len(samples), "as_of", asOf)
		return result, nil
	}

	version := newVersion(cmd.PointID, estimation, from, asOf, cmd.Trigger, s.now().UTC())

	result.Version = version
	result.Confidence = version.Confidence
	result.Flags = version.Flags
	result.Estimation = version.Estimation

	if cmd.DryRun {
		result.SkippedReason = SkippedDryRun
		return result, nil
	}

	saved, err := s.baselines.SaveNewVersion(ctx, version)
	if err != nil {
		if errors.Is(err, domain.ErrBaselineConcurrentUpdate) {
			s.log.Info("[RecalculationService] Otro proceso creó la versión vigente; no-op",
				"measuring_point_id", cmd.PointID)
			result.SkippedReason = SkippedConcurrentUpdate
			return result, nil
		}

		s.log.Error("[RecalculationService] Error guardando la versión",
			"measuring_point_id", cmd.PointID, "error", err)
		return result, domain.ErrPersistingBaseline
	}

	s.clearSkip(cmd.PointID)
	result.Created = true
	result.Version = saved

	s.log.Info("[RecalculationService] Versión de línea base creada",
		"measuring_point_id", cmd.PointID,
		"version", saved.Version,
		"trigger", saved.Trigger,
		"confidence", saved.Confidence,
		"theta_d", saved.Signal.ThetaD,
		"median_b2", saved.Signal.MedianB2,
		"passes", saved.Estimation.Passes,
		"converged", saved.Estimation.Converged,
		"excluded_ratio", saved.Estimation.ExcludedRatio,
		"flags", saved.Flags,
	)

	return result, nil
}

// RecalculateDue (UC-B2) recalcula, en secuencia, los puntos activos a los que
// les toca según RecalculationPolicy. Un error en un punto se registra y no
// detiene a los demás. Solo falla si no se pueden listar los puntos.
func (s *RecalculationService) RecalculateDue(ctx context.Context, now time.Time,
) ([]PointRecalculation, error) {
	points, err := s.history.ListActiveMeasuringPoints(ctx)
	if err != nil {
		s.log.Error("[RecalculationService] Error listando puntos activos", "error", err)
		return nil, domain.ErrConsultingHistory
	}

	summary := make([]PointRecalculation, 0, len(points))

	for _, pointID := range points {
		if ctx.Err() != nil {
			s.log.Warn("[RecalculationService] Recálculo interrumpido", "error", ctx.Err())
			break
		}

		item := PointRecalculation{PointID: pointID}

		latest, err := s.baselines.GetLatest(ctx, pointID)
		if err != nil {
			s.log.Error("[RecalculationService] Error consultando la última versión",
				"measuring_point_id", pointID, "error", err)
			item.Err = domain.ErrConsultingBaseline
			summary = append(summary, item)
			continue
		}

		item.Due, item.Trigger = s.policy.IsDue(latest, now)
		if !item.Due {
			summary = append(summary, item)
			continue
		}

		result, err := s.Recalculate(ctx, RecalculateCommand{PointID: pointID, AsOf: now, Trigger: item.Trigger})
		item.Result = &result
		item.Err = err
		summary = append(summary, item)
	}

	return summary, nil
}

// newVersion arma la versión (sin ID ni número) de una estimación sobre
// [from, asOf). La comparten Recalculate y ReplayService.
func newVersion(
	pointID int64,
	estimation domainServices.Estimation,
	from, asOf time.Time,
	trigger entities.Trigger,
	createdAt time.Time,
) entities.BaselineVersion {
	return entities.BaselineVersion{
		MeasuringPointID: pointID,
		ValidFrom:        asOf,
		HistoryFrom:      from,
		HistoryTo:        asOf,
		Signal:           estimation.Signal,
		Components:       estimation.Components,
		Confidence:       estimation.Confidence,
		ExcludedPeriods:  estimation.Exclusion.Periods,
		Estimation:       estimation.Summary,
		Flags:            estimation.Flags,
		Trigger:          trigger,
		AlgorithmVersion: features.AlgorithmVersion,
		CreatedAt:        createdAt,
	}
}

func (s *RecalculationService) logSkipOnce(pointID int64, reason SkippedReason, msg string, kv ...interface{}) {
	s.mu.Lock()
	changed := s.lastSkip[pointID] != reason
	s.lastSkip[pointID] = reason
	s.mu.Unlock()

	if changed {
		s.log.Info(msg, kv...)
	}
}

func (s *RecalculationService) clearSkip(pointID int64) {
	s.mu.Lock()
	delete(s.lastSkip, pointID)
	s.mu.Unlock()
}
