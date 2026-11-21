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

// MotionSimulateCommand es la entrada de POST .../motion-thresholds/simulate.
type MotionSimulateCommand struct {
	PointID int64
	// Patch se aplica sobre la versión vigente ahora (como el POST).
	Patch    entities.MotionPatch
	From, To time.Time // [From, To), obligatorios
}

// SimulatedMotionEpisode es un episodio de movimiento de la simulación.
type SimulatedMotionEpisode struct {
	OpenedAt  time.Time
	ClearedAt *time.Time // nil = sigue abierto al final del rango
	// Kind es el tipo de episodio; TableLevel el nivel calculado máximo y
	// SentLevel el nivel enviado máximo (motion-v2).
	Kind       entities.MotionEpisodeKind
	TableLevel entities.Level
	SentLevel  entities.Level
	// Reasons son los códigos de motivo del episodio, sin repetir, en orden de aparición.
	Reasons []entities.ReasonCode
	// TempStableFrom / TempStableTo: el último tramo de temperatura estable del episodio.
	TempStableFrom *time.Time
	TempStableTo   *time.Time
}

// MotionSimulationResult es la salida de Simulate.
type MotionSimulationResult struct {
	PointID         int64
	From, To        time.Time
	CurrentVersion  entities.MotionThresholdVersion
	Proposed        entities.MotionParams
	HoursEvaluated  int
	Episodes        []SimulatedMotionEpisode
	CurrentEpisodes []SimulatedMotionEpisode
	// NewEpisodes: propuestos que no se solapan con ninguno vigente.
	// RemovedEpisodes: vigentes que no se solapan con ninguno propuesto.
	NewEpisodes     int
	RemovedEpisodes int
}

// MotionSimulationService corre MotionAssess en memoria sobre [From, To) con
// unos parámetros propuestos y con los vigentes, sin escribir ni publicar.
// Los parámetros aplican a todo el rango y la simulación arranca sin estado
// en From (los niveles previos sí se leen de la ventana cruda; los episodios
// anteriores del historial son solo los de la propia simulación).
type MotionSimulationService struct {
	thresholds repositories.MotionThresholdRepository
	source     ports.RawMeasurementSource
	clock      ports.Clock
	log        *logger.Logger
}

func NewMotionSimulationService(thresholds repositories.MotionThresholdRepository, source ports.RawMeasurementSource,
	clock ports.Clock, log *logger.Logger) *MotionSimulationService {
	return &MotionSimulationService{thresholds: thresholds, source: source, clock: clock, log: log}
}

func (s *MotionSimulationService) Simulate(ctx context.Context, cmd MotionSimulateCommand) (MotionSimulationResult, error) {
	if cmd.PointID <= 0 {
		return MotionSimulationResult{}, domain.ErrInvalidMeasuringPointID
	}
	from, to := cmd.From.UTC(), cmd.To.UTC()
	if !from.Before(to) {
		return MotionSimulationResult{}, domain.ErrInvalidTimeRange
	}
	if to.Sub(from) > MaxSimulationRange {
		return MotionSimulationResult{}, domain.ErrSimulationRangeTooLong
	}

	current, err := s.thresholds.GetEffectiveAt(ctx, cmd.PointID, s.clock.Now().UTC())
	if err != nil {
		s.log.Error("[MotionSimulationService] Error consultando la versión vigente", "measuring_point_id", cmd.PointID, "error", err)
		return MotionSimulationResult{}, domain.ErrConsultingThresholds
	}
	if current == nil {
		return MotionSimulationResult{}, domain.ErrMotionThresholdsNotFound
	}
	proposed, err := cmd.Patch.ApplyTo(current.Params)
	if err != nil {
		return MotionSimulationResult{}, err
	}
	proposed = proposed.Normalize()
	if err := proposed.Validate(); err != nil {
		return MotionSimulationResult{}, err
	}

	start := ceilHour(from)
	winFrom, _ := entities.MotionWindow(start)
	records, err := s.source.ListRecords(ctx, cmd.PointID, winFrom, to)
	if err != nil {
		s.log.Error("[MotionSimulationService] Error leyendo raw_measurements", "measuring_point_id", cmd.PointID, "error", err)
		return MotionSimulationResult{}, domain.ErrConsultingRawMeasurements
	}

	result := MotionSimulationResult{PointID: cmd.PointID, From: from, To: to, CurrentVersion: *current, Proposed: proposed}
	result.Episodes, result.HoursEvaluated = SimulateMotionEpisodes(records, proposed, start, to)
	result.CurrentEpisodes, _ = SimulateMotionEpisodes(records, current.Params, start, to)
	result.NewEpisodes = countMotionWithoutOverlap(result.Episodes, result.CurrentEpisodes, to)
	result.RemovedEpisodes = countMotionWithoutOverlap(result.CurrentEpisodes, result.Episodes, to)
	return result, nil
}

// SimulateMotionEpisodes evalúa las horas t de [start, to) y arma los episodios.
func SimulateMotionEpisodes(records []entities.RawRecord, p entities.MotionParams, start, to time.Time) ([]SimulatedMotionEpisode, int) {
	var (
		state    = entities.InitialMotionState(p)
		episodes []SimulatedMotionEpisode
		open     *SimulatedMotionEpisode
		past     []entities.MotionEpisode
		hours    int
	)
	for t := start; t.Before(to); t = t.Add(time.Hour) {
		hours++
		ev := domainServices.MotionAssess(windowOf(records, t), state, p, t, past)
		state = ev.State
		if ep, ok := entities.EpisodeFromCleared(ev); ok {
			past = append(past, ep)
		}
		if ev.EpisodeEvent == entities.EpisodeEventOpened {
			episodes = append(episodes, SimulatedMotionEpisode{OpenedAt: t, Kind: ev.EpisodeKind,
				TableLevel: entities.LevelNormal, SentLevel: entities.LevelNormal})
			open = &episodes[len(episodes)-1]
		}
		if open == nil {
			continue
		}
		open.TableLevel = entities.MaxLevel(open.TableLevel, ev.TableLevel)
		open.SentLevel = entities.MaxLevel(open.SentLevel, ev.ComputedLevel)
		for _, r := range ev.Reasons {
			if !containsCode(open.Reasons, r.Code) {
				open.Reasons = append(open.Reasons, r.Code)
			}
		}
		if f := ev.TempStableFrom(); f != nil {
			open.TempStableFrom, open.TempStableTo = utc(f), utc(ev.TempStableTo())
		}
		if ev.EpisodeEvent == entities.EpisodeEventCleared {
			cleared := t
			open.ClearedAt = &cleared
			open = nil
		}
	}
	return episodes, hours
}

func containsCode(codes []entities.ReasonCode, c entities.ReasonCode) bool {
	for _, x := range codes {
		if x == c {
			return true
		}
	}
	return false
}

// countMotionWithoutOverlap cuenta los episodios de a que no se solapan con ninguno de b.
func countMotionWithoutOverlap(a, b []SimulatedMotionEpisode, end time.Time) int {
	n := 0
	for _, x := range a {
		overlaps := false
		for _, y := range b {
			if x.OpenedAt.Before(motionEpisodeEnd(y, end)) && y.OpenedAt.Before(motionEpisodeEnd(x, end)) {
				overlaps = true
				break
			}
		}
		if !overlaps {
			n++
		}
	}
	return n
}

func motionEpisodeEnd(e SimulatedMotionEpisode, end time.Time) time.Time {
	if e.ClearedAt != nil {
		return e.ClearedAt.Add(time.Hour)
	}
	return end.Add(time.Hour)
}
