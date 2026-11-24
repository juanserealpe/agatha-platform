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

// MaxSimulationRange es el rango máximo que acepta una simulación.
const MaxSimulationRange = 366 * 24 * time.Hour

// SimulationCreatedBy es el autor de la versión sembrada en el sandbox.
const SimulationCreatedBy = "simulation"

// SandboxFactory crea repositorios VACÍOS y aislados (en producción, en
// memoria) con una sola versión de umbrales sembrada. La simulación nunca
// toca rule_threshold_versions ni rule_evaluations.
type SandboxFactory func(seed entities.ThresholdVersion) (repositories.ThresholdRepository, repositories.RuleEvaluationRepository)

// SimulateCommand es la entrada de Simulate.
type SimulateCommand struct {
	PointID   int64
	EventType entities.EventType // vacío ⇒ FUGA
	// Patch se aplica sobre la versión vigente ahora (como el POST).
	Patch    entities.ThresholdPatch
	From, To time.Time // [From, To), obligatorios
}

// SimulatedEpisode es un episodio del Rule Engine en la simulación. Las horas
// son evaluated_at (UTC) de la fila que lo abrió, lo cerró o subió de nivel.
type SimulatedEpisode struct {
	OpenedAt       time.Time
	ClearedAt      *time.Time // nil = sigue abierto al final del rango
	MaxLevel       entities.Level
	FirstNaranjaAt *time.Time
	FirstRojoAt    *time.Time
	// Hours es el hours_above de la última fila (incluye la racha de entrada y la hora de cierre).
	Hours int
}

// SimulationResult es la salida de Simulate.
type SimulationResult struct {
	PointID   int64
	EventType entities.EventType
	From, To  time.Time
	// CurrentVersion es la versión vigente ahora (base del parche y de la comparación).
	CurrentVersion entities.ThresholdVersion
	// Proposed son los parámetros simulados (vigente + parche, normalizados).
	Proposed       entities.ThresholdParams
	HoursEvaluated int
	Episodes       []SimulatedEpisode
	ByMaxLevel     map[entities.Level]int
	// CurrentEpisodes son los episodios del mismo rango con los parámetros vigentes.
	CurrentEpisodes []SimulatedEpisode
	// NewEpisodes: propuestos que no se solapan con ninguno vigente.
	// RemovedEpisodes: vigentes que no se solapan con ninguno propuesto.
	NewEpisodes     int
	RemovedEpisodes int
}

// SimulationService corre el histórico de feature_evaluations con unos
// parámetros propuestos SIN guardar ni publicar: reusa EvaluateIndicatorWithoutPublishing
// sobre un sandbox, así que la interpretación es EXACTAMENTE la de producción.
//
// Los parámetros se aplican a TODO el rango (no a la línea de tiempo real de
// versiones) y la simulación arranca sin historial en From: las primeras
// horas no ven las rachas previas al rango.
type SimulationService struct {
	thresholds repositories.ThresholdRepository
	source     ports.IndicatorSource
	sandbox    SandboxFactory
	clock      ports.Clock
	log        *logger.Logger
	batchSize  int
}

func NewSimulationService(
	thresholds repositories.ThresholdRepository,
	source ports.IndicatorSource,
	sandbox SandboxFactory,
	clock ports.Clock,
	log *logger.Logger,
) *SimulationService {
	return &SimulationService{
		thresholds: thresholds,
		source:     source,
		sandbox:    sandbox,
		clock:      clock,
		log:        log,
		batchSize:  defaultBackfillBatchSize,
	}
}

// Simulate evalúa [From, To) con la versión vigente + el parche y lo compara
// con la versión vigente sola.
func (s *SimulationService) Simulate(ctx context.Context, cmd SimulateCommand) (SimulationResult, error) {
	if cmd.PointID <= 0 {
		return SimulationResult{}, domain.ErrInvalidMeasuringPointID
	}
	eventType := normalizeEventType(cmd.EventType)
	if eventType != entities.EventTypeLeak {
		return SimulationResult{}, domain.ErrInvalidEventType
	}
	from, to := cmd.From.UTC(), cmd.To.UTC()
	if from.IsZero() || to.IsZero() || !from.Before(to) {
		return SimulationResult{}, domain.ErrInvalidTimeRange
	}
	if to.Sub(from) > MaxSimulationRange {
		return SimulationResult{}, domain.ErrSimulationRangeTooLong
	}

	current, err := s.thresholds.GetEffectiveAt(ctx, cmd.PointID, eventType, s.clock.Now().UTC())
	if err != nil {
		s.log.Error("[SimulationService] Error consultando los umbrales vigentes", "measuring_point_id", cmd.PointID, "error", err)
		return SimulationResult{}, domain.ErrConsultingThresholds
	}
	if current == nil {
		return SimulationResult{}, domain.ErrThresholdsNotFound
	}

	proposed := cmd.Patch.ApplyTo(current.ThresholdParams).Normalize()
	if err := proposed.Validate(); err != nil {
		return SimulationResult{}, err
	}

	hours, err := s.readHours(ctx, cmd.PointID, from, to)
	if err != nil {
		return SimulationResult{}, err
	}

	proposedRows, err := s.run(ctx, cmd.PointID, eventType, from, proposed, hours)
	if err != nil {
		return SimulationResult{}, err
	}
	currentRows, err := s.run(ctx, cmd.PointID, eventType, from, current.ThresholdParams, hours)
	if err != nil {
		return SimulationResult{}, err
	}

	result := SimulationResult{
		PointID:         cmd.PointID,
		EventType:       eventType,
		From:            from,
		To:              to,
		CurrentVersion:  *current,
		Proposed:        proposed,
		HoursEvaluated:  len(proposedRows),
		Episodes:        EpisodesFromRows(proposedRows),
		ByMaxLevel:      map[entities.Level]int{},
		CurrentEpisodes: EpisodesFromRows(currentRows),
	}
	for _, e := range result.Episodes {
		result.ByMaxLevel[e.MaxLevel]++
	}
	result.NewEpisodes = countWithoutOverlap(result.Episodes, result.CurrentEpisodes)
	result.RemovedEpisodes = countWithoutOverlap(result.CurrentEpisodes, result.Episodes)

	s.log.Info("[SimulationService] Simulación de umbrales",
		"measuring_point_id", cmd.PointID, "from", from, "to", to, "hours", result.HoursEvaluated,
		"episodes", len(result.Episodes), "current_episodes", len(result.CurrentEpisodes),
		"new_episodes", result.NewEpisodes, "removed_episodes", result.RemovedEpisodes)

	return result, nil
}

// readHours lee una sola vez las horas CALCULATED del rango (las dos corridas
// usan las mismas).
func (s *SimulationService) readHours(ctx context.Context, pointID int64, from, to time.Time) ([]ports.CalculatedIndicator, error) {
	var hours []ports.CalculatedIndicator
	var after time.Time
	for {
		batch, err := s.source.ListCalculated(ctx, pointID, from, to, after, s.batchSize)
		if err != nil {
			s.log.Error("[SimulationService] Error leyendo feature_evaluations", "measuring_point_id", pointID, "error", err)
			return nil, domain.ErrConsultingIndicators
		}
		hours = append(hours, batch...)
		if len(batch) < s.batchSize {
			return hours, nil
		}
		after = batch[len(batch)-1].EvaluatedAt
	}
}

// run interpreta las horas con params en un sandbox nuevo y devuelve las filas
// que habría guardado, en orden ascendente.
func (s *SimulationService) run(
	ctx context.Context,
	pointID int64,
	eventType entities.EventType,
	from time.Time,
	params entities.ThresholdParams,
	hours []ports.CalculatedIndicator,
) ([]entities.RuleEvaluation, error) {
	now := s.clock.Now().UTC()
	thresholds, evaluations := s.sandbox(entities.ThresholdVersion{
		MeasuringPointID: pointID,
		EventType:        eventType,
		Version:          1,
		ThresholdParams:  params,
		ValidFrom:        from,
		Reason:           "simulación",
		CreatedBy:        SimulationCreatedBy,
		CreatedAt:        now,
	})
	// Sin publisher: nunca publica.
	evaluator := NewEvaluationService(thresholds, evaluations, NewThresholdService(thresholds, s.clock, s.log), nil, s.clock, s.log)

	rows := make([]entities.RuleEvaluation, 0, len(hours))
	for _, h := range hours {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		i := h.I
		r, err := evaluator.EvaluateIndicatorWithoutPublishing(ctx, IndicatorEvaluatedNotice{
			MeasuringPointID: pointID,
			WindowHours:      WindowHours24,
			EvaluatedAt:      h.EvaluatedAt,
			Status:           StatusCalculated,
			I:                &i,
			LowConfidence:    h.LowConfidence,
			Trigger:          TriggerSimulation,
		})
		if err != nil {
			return nil, err
		}
		if r.Outcome == OutcomeSaved {
			rows = append(rows, r.Evaluation)
		}
	}
	return rows, nil
}

// EpisodesFromRows agrupa filas ascendentes de un punto en episodios: desde
// la fila OPENED hasta la CLEARED (incluida).
func EpisodesFromRows(rows []entities.RuleEvaluation) []SimulatedEpisode {
	var out []SimulatedEpisode
	open := -1
	for _, r := range rows {
		if r.EpisodeEvent == entities.EpisodeEventOpened {
			out = append(out, SimulatedEpisode{OpenedAt: r.EvaluatedAt, MaxLevel: entities.LevelNormal})
			open = len(out) - 1
		}
		if open < 0 {
			continue
		}

		e := &out[open]
		at := r.EvaluatedAt
		e.Hours = r.HoursAbove
		e.MaxLevel = entities.MaxLevel(e.MaxLevel, r.Level)
		if e.FirstNaranjaAt == nil && r.Level.AtLeast(entities.LevelNaranja) {
			e.FirstNaranjaAt = &at
		}
		if e.FirstRojoAt == nil && r.Level == entities.LevelRojo {
			e.FirstRojoAt = &at
		}
		if r.EpisodeEvent == entities.EpisodeEventCleared {
			e.ClearedAt = &at
			open = -1
		}
	}
	return out
}

// countWithoutOverlap cuenta los episodios de a que no se solapan con ninguno
// de b (un episodio abierto se extiende hasta el infinito).
func countWithoutOverlap(a, b []SimulatedEpisode) int {
	n := 0
	for _, x := range a {
		overlaps := false
		for _, y := range b {
			if episodesOverlap(x, y) {
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

func episodesOverlap(a, b SimulatedEpisode) bool {
	aEndsAfterBStarts := a.ClearedAt == nil || !a.ClearedAt.Before(b.OpenedAt)
	bEndsAfterAStarts := b.ClearedAt == nil || !b.ClearedAt.Before(a.OpenedAt)
	return aEndsAfterBStarts && bEndsAfterAStarts
}
