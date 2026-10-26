package services

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
	domainServices "github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// ProductionEnvironment es el valor de AGATHA_ENV que bloquea -rebuild.
const ProductionEnvironment = "production"

const defaultReplayBatchSize = 500

// ReplayConfig son los parámetros del catch-up y el backfill.
type ReplayConfig struct {
	Environment string
	// BatchSize son las filas de rule_evaluations por consulta (≤ 0 ⇒ 500).
	BatchSize int
}

// PointReplay es el resumen de un punto.
type PointReplay struct {
	PointID int64
	// WithoutHistory: sin estado de procesamiento; el catch-up lo saltea
	// (su histórico lo carga alert_management -backfill, que no publica avisos).
	WithoutHistory bool
	Hours          int
	Messages       int
	// Notifications son los avisos publicados (siempre 0 en el backfill).
	Notifications int
	Opened        int
	Reactivated   int
	Finished      int
	Err           error
}

// BackfillCommand es la entrada del backfill.
type BackfillCommand struct {
	PointID int64     // 0 ⇒ todos los puntos de rule_evaluations
	From    time.Time // cero ⇒ desde la última hora procesada (o el inicio)
	To      time.Time // cero ⇒ hasta la última (exclusivo)
	// Rebuild borra TODO lo del punto (alertas, colores, mensajes,
	// observaciones, estado) y lo reconstruye. Nunca en production.
	Rebuild bool
}

// BackfillResult es el resumen del backfill.
type BackfillResult struct {
	PointReplay
	Deleted int64
	Points  []PointReplay
}

// ReplayService recorre rule_evaluations (solo lectura) con EXACTAMENTE el
// mismo caso de uso que la evaluación en vivo:
//   - CatchUp (al arrancar y cada hora en el minuto 55, después del 50 del
//     Rule Engine): las horas posteriores a la última procesada, publicando avisos.
//     Cubre los eventos perdidos del bus.
//   - Backfill (cmd/alert_management -backfill): el histórico, sin publicar avisos.
//
// Las horas sin cálculo no tienen fila: el salto de evaluated_at se procesa
// como horas NO_CALCULABLE (pausan las cuentas igual que en vivo).
type ReplayService struct {
	processor *ProcessingService
	levels    ports.LevelSource
	reset     ports.AlertReset
	cfg       ReplayConfig
	log       *logger.Logger
	states    stateReader
}

// stateReader lee el estado de procesamiento fuera de una escritura.
type stateReader interface {
	Transaction(ctx context.Context, fn func(tx ports.AlertTx) error) error
}

// ReplayStore es el store que necesita el replay: transacciones (para leer el
// estado) y el borrado del rebuild.
type ReplayStore interface {
	ports.AlertStore
	ports.AlertReset
}

func NewReplayService(processor *ProcessingService, levels ports.LevelSource, store ReplayStore, cfg ReplayConfig, log *logger.Logger) *ReplayService {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = defaultReplayBatchSize
	}
	return &ReplayService{processor: processor, levels: levels, reset: store, states: store, cfg: cfg, log: log}
}

// CatchUp procesa, publicando avisos, las horas que faltan de cada punto con
// historial. Un error en un punto lo detiene (el orden importa) y no a los
// demás; solo falla si no se pueden listar los puntos.
func (s *ReplayService) CatchUp(ctx context.Context) ([]PointReplay, error) {
	points, err := s.levels.ListPoints(ctx)
	if err != nil {
		s.log.Error("[ReplayService] Error listando los puntos de rule_evaluations", "error", err)
		return nil, domain.ErrConsultingRuleEvaluations
	}
	slices.Sort(points)

	summary := make([]PointReplay, 0, len(points))
	for _, pointID := range points {
		if ctx.Err() != nil {
			break
		}
		last, err := s.lastEvaluatedAt(ctx, pointID)
		if err != nil {
			summary = append(summary, PointReplay{PointID: pointID, Err: err})
			continue
		}
		if last == nil {
			summary = append(summary, PointReplay{PointID: pointID, WithoutHistory: true})
			continue
		}
		item := s.replayPoint(ctx, pointID, *last, time.Time{}, entities.SourceCatchUp)
		if item.Hours > 0 {
			s.log.Warn("[ReplayService] Horas procesadas por el catch-up (evento perdido o no publicado)",
				"measuring_point_id", pointID, "hours", item.Hours, "notifications", item.Notifications)
		}
		summary = append(summary, item)
	}
	return summary, nil
}

// Backfill reconstruye el histórico sin publicar avisos. Idempotente: retoma desde
// la última hora procesada. Un error aborta (repetirlo retoma).
func (s *ReplayService) Backfill(ctx context.Context, cmd BackfillCommand) (BackfillResult, error) {
	result := BackfillResult{}
	if cmd.Rebuild && strings.EqualFold(strings.TrimSpace(s.cfg.Environment), ProductionEnvironment) {
		return result, domain.ErrRebuildForbidden
	}
	if cmd.PointID < 0 {
		return result, domain.ErrInvalidMeasuringPointID
	}
	if !cmd.From.IsZero() && !cmd.To.IsZero() && !cmd.From.Before(cmd.To) {
		return result, domain.ErrInvalidTimeRange
	}

	points := []int64{cmd.PointID}
	if cmd.PointID == 0 {
		var err error
		if points, err = s.levels.ListPoints(ctx); err != nil {
			s.log.Error("[ReplayService] Error listando los puntos de rule_evaluations", "error", err)
			return result, domain.ErrConsultingRuleEvaluations
		}
		slices.Sort(points)
	}

	for _, pointID := range points {
		if cmd.Rebuild {
			deleted, err := s.reset.ResetPoint(ctx, pointID, entities.EventTypeLeak)
			if err != nil {
				s.log.Error("[ReplayService] Error borrando para el rebuild", "measuring_point_id", pointID, "error", err)
				return result, domain.ErrPersistingAlerts
			}
			result.Deleted += deleted
			s.log.Warn("[ReplayService] Rebuild: alertas del punto borradas", "measuring_point_id", pointID, "deleted", deleted)
		}

		after := cmd.From.Add(-time.Nanosecond)
		if cmd.From.IsZero() {
			after = time.Time{}
		}
		last, err := s.lastEvaluatedAt(ctx, pointID)
		if err != nil {
			return result, err
		}
		if last != nil && last.After(after) {
			after = *last
		}

		item := s.replayPoint(ctx, pointID, after, cmd.To.UTC(), entities.SourceBackfill)
		result.Points = append(result.Points, item)
		result.add(item)
		if item.Err != nil {
			return result, item.Err
		}
	}
	return result, nil
}

func (s *ReplayService) lastEvaluatedAt(ctx context.Context, pointID int64) (*time.Time, error) {
	var last *time.Time
	err := s.states.Transaction(ctx, func(tx ports.AlertTx) error {
		st, err := tx.GetState(ctx, pointID, entities.EventTypeLeak)
		if err != nil || st == nil {
			return err
		}
		last = st.LastEvaluatedAt
		return nil
	})
	if err != nil {
		s.log.Error("[ReplayService] Error leyendo el estado de procesamiento", "measuring_point_id", pointID, "error", err)
		return nil, domain.ErrConsultingAlerts
	}
	return last, nil
}

func (s *ReplayService) replayPoint(ctx context.Context, pointID int64, after, to time.Time, source entities.Source) PointReplay {
	item := PointReplay{PointID: pointID}
	for {
		if err := ctx.Err(); err != nil {
			item.Err = err
			return item
		}
		batch, err := s.levels.ListAfter(ctx, pointID, after, to, s.cfg.BatchSize)
		if err != nil {
			s.log.Error("[ReplayService] Error leyendo rule_evaluations", "measuring_point_id", pointID, "after", after, "error", err)
			item.Err = domain.ErrConsultingRuleEvaluations
			return item
		}
		for _, n := range batch {
			r, err := s.processor.Process(ctx, n, source)
			if err != nil {
				item.Err = err
				return item
			}
			item.record(r)
		}
		if len(batch) > 0 {
			after = batch[len(batch)-1].EvaluatedAt
		}
		if len(batch) < s.cfg.BatchSize {
			return item
		}
	}
}

func (p *PointReplay) record(r ProcessResult) {
	p.Hours += r.Hours
	p.Messages += r.Messages
	p.Notifications += r.Notifications
	for _, o := range r.Decisions {
		switch o {
		case domainServices.OutcomeOpened:
			p.Opened++
		case domainServices.OutcomeReactivated:
			p.Reactivated++
		case domainServices.OutcomeFinished:
			p.Finished++
		}
	}
}

func (p *PointReplay) add(o PointReplay) {
	p.Hours += o.Hours
	p.Messages += o.Messages
	p.Notifications += o.Notifications
	p.Opened += o.Opened
	p.Reactivated += o.Reactivated
	p.Finished += o.Finished
}
