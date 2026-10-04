package services

import (
	"context"
	"sort"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/repositories"
	domainServices "github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/features"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// ReplayService reconstruye, sin escribir en la BD, las versiones de línea
// base que habrían existido en el pasado. Lo usa la herramienta de
// reproducción (cmd/replay) vía baseline.Replayer.
//
// No tiene BaselineRepository: la imposibilidad de persistir es estructural.
type ReplayService struct {
	history   repositories.HistoryReader
	estimator domainServices.RobustBaselineEstimator
	policy    domainServices.RecalculationPolicy
	cfg       domainServices.Config
	log       *logger.Logger
}

func NewReplayService(history repositories.HistoryReader, cfg domainServices.Config, log *logger.Logger) *ReplayService {
	return &ReplayService{
		history:   history,
		estimator: domainServices.NewRobustBaselineEstimator(cfg),
		policy:    domainServices.NewRecalculationPolicy(cfg),
		cfg:       cfg,
		log:       log,
	}
}

// SimulateVersions recorre [from, to) con un reloj virtual horario (horas en
// punto UTC) y, en cada hora, aplica la RecalculationPolicy real sobre la
// última versión simulada; si toca, estima con SOLO los datos de
// [hora − HistoryDays, hora). Devuelve las versiones en orden de ValidFrom
// (ID = 0, número secuencial, ValidTo cerrado por la siguiente) y el primer
// registro de B2 del dispositivo. Lee el histórico una sola vez.
//
// Aproximación: producción recalcula en el minuto BASELINE_TICK_MINUTE; la
// simulación lo hace en la hora en punto.
func (s *ReplayService) SimulateVersions(ctx context.Context, pointID int64, from, to time.Time,
) ([]entities.BaselineVersion, *time.Time, error) {
	if pointID <= 0 {
		return nil, nil, domain.ErrInvalidMeasuringPointID
	}

	start := ceilHour(from.UTC())
	end := to.UTC()

	samples, firstSeen, err := s.load(ctx, pointID, start.Add(-s.cfg.HistoryWindow()), end)
	if err != nil {
		return nil, nil, err
	}

	versions := []entities.BaselineVersion{}
	var latest *entities.BaselineVersion

	for tick := start; tick.Before(end); tick = tick.Add(time.Hour) {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}

		due, trigger := s.policy.IsDue(latest, tick)
		if !due {
			continue
		}

		version, ok := s.estimate(pointID, samples, tick, trigger)
		if !ok {
			continue
		}

		version.Version = len(versions) + 1
		if n := len(versions); n > 0 {
			validTo := tick
			versions[n-1].ValidTo = &validTo
		}
		versions = append(versions, version)
		latest = &versions[len(versions)-1]
	}

	s.log.Info("[ReplayService] Versiones simuladas",
		"measuring_point_id", pointID, "from", start, "to", end, "versions", len(versions))

	return versions, firstSeen, nil
}

// ComputeAt calcula una sola versión con los datos de [asOf − HistoryDays,
// asOf) (modo "línea base fija"). Version = nil si el histórico es insuficiente.
func (s *ReplayService) ComputeAt(ctx context.Context, pointID int64, asOf time.Time) (EffectiveResult, error) {
	if pointID <= 0 {
		return EffectiveResult{}, domain.ErrInvalidMeasuringPointID
	}

	asOf = asOf.UTC()

	samples, firstSeen, err := s.load(ctx, pointID, asOf.Add(-s.cfg.HistoryWindow()), asOf)
	if err != nil {
		return EffectiveResult{}, err
	}

	result := EffectiveResult{FirstSeenAt: firstSeen}

	if version, ok := s.estimate(pointID, samples, asOf, entities.TriggerManual); ok {
		version.Version = 1
		result.Version = &version
	}

	return result, nil
}

// estimate ejecuta el estimador real con los samples de [asOf − HistoryDays, asOf).
func (s *ReplayService) estimate(pointID int64, samples []features.Sample, asOf time.Time, trigger entities.Trigger,
) (entities.BaselineVersion, bool) {
	from := asOf.Add(-s.cfg.HistoryWindow())

	estimation := s.estimator.Estimate(between(samples, from, asOf), from, asOf)
	if estimation.Status == domainServices.EstimationInsufficientHistory {
		return entities.BaselineVersion{}, false
	}

	return newVersion(pointID, estimation, from, asOf, trigger, asOf), true
}

func (s *ReplayService) load(ctx context.Context, pointID int64, from, to time.Time,
) ([]features.Sample, *time.Time, error) {
	raw, err := s.history.GetSamples(ctx, pointID, from, to)
	if err != nil {
		s.log.Error("[ReplayService] Error consultando el histórico", "measuring_point_id", pointID, "error", err)
		return nil, nil, domain.ErrConsultingHistory
	}

	firstSeen, err := s.history.GetFirstSeenAt(ctx, pointID)
	if err != nil {
		s.log.Error("[ReplayService] Error consultando el primer registro", "measuring_point_id", pointID, "error", err)
		return nil, nil, domain.ErrConsultingHistory
	}

	return features.Normalize(raw), firstSeen, nil
}

// between devuelve la sub-secuencia [from, to) de samples ordenados (sin copiar).
func between(samples []features.Sample, from, to time.Time) []features.Sample {
	lo := sort.Search(len(samples), func(i int) bool { return !samples[i].T.Before(from) })
	hi := sort.Search(len(samples), func(i int) bool { return !samples[i].T.Before(to) })
	return samples[lo:hi]
}

func ceilHour(t time.Time) time.Time {
	floor := t.Truncate(time.Hour)
	if floor.Equal(t) {
		return t
	}
	return floor.Add(time.Hour)
}
