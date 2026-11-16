package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/repositories"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// ProductionEnvironment es el valor de AGATHA_ENV que bloquea -rebuild.
const ProductionEnvironment = "production"

const defaultBackfillBatchSize = 500

// BackfillConfig son los parámetros del backfill.
type BackfillConfig struct {
	// Environment es AGATHA_ENV: con "production" no se permite Rebuild.
	Environment string
	// BatchSize son las horas leídas de feature_evaluations por consulta (≤ 0 ⇒ 500).
	BatchSize int
}

// BackfillCommand es la entrada del backfill.
type BackfillCommand struct {
	PointID int64     // 0 ⇒ todos los puntos con horas calculadas en el rango
	From    time.Time // cero ⇒ desde la primera hora
	To      time.Time // cero ⇒ hasta la última (fin exclusivo)
	// Rebuild borra rule_evaluations del rango/punto y lo recalcula. Nunca en production.
	Rebuild bool

	// ResetVersions reconstruye el historial de versiones del punto antes del
	// rebuild: toma la versión vigente HOY, borra TODAS las versiones FUGA del
	// punto e inserta una única versión 1 con esos mismos parámetros y
	// valid_from = su primera medición. Así el histórico completo se interpreta
	// con los parámetros vigentes.
	//
	// Es la SEGUNDA excepción deliberada a la no retroactividad de los umbrales
	// (la primera es la v1 sembrada por SeedInitialThresholds), acotada a UN
	// punto y bloqueada con AGATHA_ENV=production. Exige Rebuild, PointID,
	// ConfirmPointID == PointID y el historial completo (sin From ni To).
	ResetVersions bool
	// ConfirmPointID es el valor de -confirm: debe repetir PointID.
	ConfirmPointID int64
}

// PointBackfill es el resumen de un punto.
type PointBackfill struct {
	PointID        int64
	Hours          int // horas CALCULATED de 24 h leídas
	Saved          int
	Duplicates     int
	NoThresholds   int
	Late           int
	EpisodesOpened int
	EpisodesClosed int
	HoursByLevel   map[entities.Level]int // solo de las horas guardadas en esta corrida
	First, Last    *time.Time
}

// BackfillResult es el resumen de la corrida (totales + por punto).
type BackfillResult struct {
	PointBackfill
	Deleted int64
	Points  []PointBackfill

	// Solo con ResetVersions: cuántas versiones se colapsaron, el número de la
	// versión cuyos parámetros se aplicaron al histórico y el valid_from de la
	// versión 1 nueva.
	VersionsCollapsed  int
	ResetSourceVersion int
	ResetValidFrom     *time.Time
}

// BackfillService interpreta el histórico de feature_evaluations (24 h,
// CALCULATED) con EXACTAMENTE el mismo caso de uso que la evaluación en vivo,
// en modo sin publicación: NUNCA publica. Por punto y en orden ascendente de
// evaluated_at, en lotes. Idempotente: las horas ya interpretadas se saltean.
//
// Orden: primero el backfill de Processing (cmd/processing -backfill), después
// este, y recién entonces habilitar rule_engine en AGATHA_MODULES.
type BackfillService struct {
	evaluator   *EvaluationService
	source      ports.IndicatorSource
	evaluations repositories.RuleEvaluationRepository
	cfg         BackfillConfig
	log         *logger.Logger

	// resetter y clock solo se usan con ResetVersions (WithHistoryResetter).
	resetter repositories.HistoryResetter
	clock    ports.Clock
}

func NewBackfillService(
	evaluator *EvaluationService,
	source ports.IndicatorSource,
	evaluations repositories.RuleEvaluationRepository,
	cfg BackfillConfig,
	log *logger.Logger,
) *BackfillService {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = defaultBackfillBatchSize
	}
	return &BackfillService{evaluator: evaluator, source: source, evaluations: evaluations, cfg: cfg, log: log}
}

// WithHistoryResetter habilita BackfillCommand.ResetVersions. Sin él, pedir el
// reinicio falla con ErrResetUnavailable (nunca se ignora en silencio).
func (s *BackfillService) WithHistoryResetter(resetter repositories.HistoryResetter, clock ports.Clock) *BackfillService {
	s.resetter, s.clock = resetter, clock
	return s
}

// Backfill corre el histórico. Un error aborta la corrida: lo ya guardado
// queda (es idempotente) y repetirla retoma donde quedó.
func (s *BackfillService) Backfill(ctx context.Context, cmd BackfillCommand) (BackfillResult, error) {
	result := BackfillResult{PointBackfill: PointBackfill{HoursByLevel: map[entities.Level]int{}}}

	if (cmd.Rebuild || cmd.ResetVersions) && strings.EqualFold(strings.TrimSpace(s.cfg.Environment), ProductionEnvironment) {
		return result, domain.ErrRebuildForbidden
	}
	if err := s.validateReset(cmd); err != nil {
		return result, err
	}
	if !cmd.From.IsZero() && !cmd.To.IsZero() && !cmd.From.Before(cmd.To) {
		return result, domain.ErrInvalidTimeRange
	}
	if cmd.PointID < 0 {
		return result, domain.ErrInvalidMeasuringPointID
	}
	from, to := cmd.From.UTC(), cmd.To.UTC()

	points := []int64{cmd.PointID}
	if cmd.PointID == 0 {
		var err error
		if points, err = s.source.ListPoints(ctx, from, to); err != nil {
			s.log.Error("[BackfillService] Error listando los puntos de feature_evaluations", "error", err)
			return result, domain.ErrConsultingIndicators
		}
	}

	if cmd.ResetVersions {
		if err := s.resetVersions(ctx, cmd.PointID, &result); err != nil {
			return result, err
		}
	}

	if cmd.Rebuild {
		var pointFilter *int64
		if cmd.PointID > 0 {
			pointFilter = &cmd.PointID
		}
		deleted, err := s.evaluations.DeleteRange(ctx, pointFilter, entities.EventTypeLeak, from, to)
		if err != nil {
			s.log.Error("[BackfillService] Error borrando rule_evaluations para el rebuild", "error", err)
			return result, domain.ErrPersistingRuleEvaluations
		}
		result.Deleted += deleted
		s.log.Warn("[BackfillService] Rebuild: filas de rule_evaluations borradas",
			"measuring_point_id", cmd.PointID, "from", from, "to", to, "deleted", deleted)
	}

	for _, pointID := range points {
		summary, err := s.backfillPoint(ctx, pointID, from, to)
		result.Points = append(result.Points, summary)
		result.add(summary)
		if err != nil {
			return result, err
		}
	}

	s.log.Info("[BackfillService] Backfill del Rule Engine terminado",
		"points", len(points), "hours", result.Hours, "saved", result.Saved, "duplicates", result.Duplicates,
		"no_thresholds", result.NoThresholds, "episodes_opened", result.EpisodesOpened,
		"episodes_closed", result.EpisodesClosed, "deleted", result.Deleted)

	return result, nil
}

// validateReset aplica las guardas de ResetVersions (CA-11) antes de escribir
// nada: exige -rebuild, un punto concreto, la confirmación explícita de ese
// mismo punto y el historial completo.
func (s *BackfillService) validateReset(cmd BackfillCommand) error {
	if !cmd.ResetVersions {
		return nil
	}
	switch {
	case !cmd.Rebuild:
		return domain.ErrResetRequiresRebuild
	case cmd.PointID <= 0:
		return domain.ErrResetRequiresPoint
	case cmd.ConfirmPointID != cmd.PointID:
		return domain.ErrResetNotConfirmed
	case !cmd.From.IsZero() || !cmd.To.IsZero():
		return domain.ErrResetFullHistory
	case s.resetter == nil || s.clock == nil:
		return domain.ErrResetUnavailable
	}
	return nil
}

// resetVersions colapsa el historial de versiones FUGA del punto en una sola
// versión 1, en UNA transacción: lee (y bloquea) las versiones, toma la
// vigente hoy, borra las filas de rule_evaluations del punto (la FK a la
// versión es RESTRICT, así que van ANTES que las versiones), borra las
// versiones e inserta la v1 con los parámetros de la vigente y valid_from = la
// primera medición del punto (now si no tiene). Si algo falla no queda nada
// escrito. El rebuild normal recalcula después las filas.
func (s *BackfillService) resetVersions(ctx context.Context, pointID int64, result *BackfillResult) error {
	now := s.clock.Now().UTC()

	err := s.resetter.Transaction(ctx, func(tx repositories.HistoryResetTx) error {
		versions, err := tx.LockVersions(ctx, pointID, entities.EventTypeLeak)
		if err != nil {
			return err
		}

		var current *entities.ThresholdVersion
		for n := range versions {
			switch {
			case versions[n].ValidFrom.After(now):
				return domain.ErrResetFutureVersions
			case versions[n].IsEffectiveAt(now):
				current = &versions[n]
			}
		}
		if current == nil {
			return domain.ErrThresholdsNotFound
		}

		validFrom := now
		first, err := tx.FirstMeasurementAt(ctx, pointID)
		if err != nil {
			return err
		}
		if first != nil {
			validFrom = first.UTC()
		}

		deleted, err := tx.DeleteEvaluations(ctx, pointID, entities.EventTypeLeak)
		if err != nil {
			return err
		}
		if _, err := tx.DeleteVersions(ctx, pointID, entities.EventTypeLeak); err != nil {
			return err
		}
		if _, err := tx.InsertVersion(ctx, entities.ThresholdVersion{
			MeasuringPointID: pointID,
			EventType:        entities.EventTypeLeak,
			Version:          1,
			// ApplyTo con un parche vacío copia los parámetros (y sus punteros).
			ThresholdParams: entities.ThresholdPatch{}.ApplyTo(current.ThresholdParams),
			ValidFrom:       validFrom,
			Reason:          fmt.Sprintf("Reconstrucción: parámetros de la versión %d aplicados al histórico", current.Version),
			CreatedBy:       entities.SystemCreatedBy,
			CreatedAt:       now,
		}); err != nil {
			return err
		}

		result.VersionsCollapsed, result.ResetSourceVersion = len(versions), current.Version
		result.ResetValidFrom = &validFrom
		result.Deleted += deleted
		return nil
	})
	if err == nil {
		s.log.Warn("[BackfillService] Reset de versiones: historial de umbrales colapsado en la versión 1 (excepción a la no retroactividad)",
			"measuring_point_id", pointID, "versions_collapsed", result.VersionsCollapsed,
			"source_version", result.ResetSourceVersion, "valid_from", result.ResetValidFrom)
		return nil
	}

	*result = BackfillResult{PointBackfill: PointBackfill{HoursByLevel: map[entities.Level]int{}}}
	switch {
	case errors.Is(err, domain.ErrResetFutureVersions), errors.Is(err, domain.ErrThresholdsNotFound):
		return err
	case errors.Is(err, repositories.ErrConflict):
		return domain.ErrThresholdConflict
	default:
		s.log.Error("[BackfillService] Error reconstruyendo las versiones de umbrales (rollback)",
			"measuring_point_id", pointID, "error", err)
		return domain.ErrPersistingThresholds
	}
}

func (s *BackfillService) backfillPoint(ctx context.Context, pointID int64, from, to time.Time) (PointBackfill, error) {
	summary := PointBackfill{PointID: pointID, HoursByLevel: map[entities.Level]int{}}
	var after time.Time

	for {
		if err := ctx.Err(); err != nil {
			s.log.Warn("[BackfillService] Backfill interrumpido", "measuring_point_id", pointID, "error", err)
			return summary, err
		}

		batch, err := s.source.ListCalculated(ctx, pointID, from, to, after, s.cfg.BatchSize)
		if err != nil {
			s.log.Error("[BackfillService] Error leyendo feature_evaluations",
				"measuring_point_id", pointID, "after", after, "error", err)
			return summary, domain.ErrConsultingIndicators
		}

		for _, h := range batch {
			i := h.I
			r, err := s.evaluator.EvaluateIndicatorWithoutPublishing(ctx, IndicatorEvaluatedNotice{
				MeasuringPointID: pointID,
				WindowHours:      WindowHours24,
				EvaluatedAt:      h.EvaluatedAt,
				Status:           StatusCalculated,
				I:                &i,
				DPl:              h.DPl,
				DPlLoaded:        true,
				LowConfidence:    h.LowConfidence,
				Trigger:          TriggerBackfill,
			})
			if err != nil {
				return summary, err
			}
			summary.record(h.EvaluatedAt, r)
		}

		if len(batch) > 0 {
			after = batch[len(batch)-1].EvaluatedAt
			s.log.Info("[BackfillService] Progreso",
				"measuring_point_id", pointID, "hours", summary.Hours, "until", after,
				"normal", summary.HoursByLevel[entities.LevelNormal], "amarillo", summary.HoursByLevel[entities.LevelAmarillo],
				"naranja", summary.HoursByLevel[entities.LevelNaranja], "rojo", summary.HoursByLevel[entities.LevelRojo],
				"episodes_opened", summary.EpisodesOpened, "episodes_closed", summary.EpisodesClosed)
		}
		if len(batch) < s.cfg.BatchSize {
			return summary, nil
		}
	}
}

func (p *PointBackfill) record(at time.Time, r EvaluationResult) {
	p.Hours++
	if p.First == nil {
		first := at
		p.First = &first
	}
	last := at
	p.Last = &last

	switch r.Outcome {
	case OutcomeDuplicate:
		p.Duplicates++
	case OutcomeNoThresholds:
		p.NoThresholds++
	case OutcomeSaved:
		p.Saved++
		p.HoursByLevel[r.Evaluation.Level]++
		if r.Late {
			p.Late++
		}
		switch r.Evaluation.EpisodeEvent {
		case entities.EpisodeEventOpened:
			p.EpisodesOpened++
		case entities.EpisodeEventCleared:
			p.EpisodesClosed++
		}
	}
}

func (p *PointBackfill) add(o PointBackfill) {
	p.Hours += o.Hours
	p.Saved += o.Saved
	p.Duplicates += o.Duplicates
	p.NoThresholds += o.NoThresholds
	p.Late += o.Late
	p.EpisodesOpened += o.EpisodesOpened
	p.EpisodesClosed += o.EpisodesClosed
	for level, n := range o.HoursByLevel {
		p.HoursByLevel[level] += n
	}
	if o.First != nil && (p.First == nil || o.First.Before(*p.First)) {
		p.First = o.First
	}
	if o.Last != nil && (p.Last == nil || o.Last.After(*p.Last)) {
		p.Last = o.Last
	}
}
