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
	// BatchSize son las filas de rule_evaluations / motion_evaluations por consulta (≤ 0 ⇒ 500).
	BatchSize int
}

// PointReplay es el resumen de un punto.
type PointReplay struct {
	PointID int64
	// WithoutHistory: sin estado de procesamiento de ningún flujo; el catch-up
	// lo saltea (su histórico lo carga alert_management -backfill).
	WithoutHistory bool
	Hours          int
	// MotionHours son las horas de movimiento procesadas (incluidas en Hours).
	MotionHours int
	Messages    int
	// Notifications son los avisos publicados (siempre 0 en el backfill).
	Notifications int
	Opened        int
	Reactivated   int
	Finished      int
	// Reset es lo que -rebuild borró del punto.
	Reset ports.ResetCounts
	Err   error
}

// BackfillCommand es la entrada del backfill.
type BackfillCommand struct {
	PointID int64     // 0 ⇒ todos los puntos de rule_evaluations y motion_evaluations
	From    time.Time // cero ⇒ desde la última hora procesada (o el inicio)
	To      time.Time // cero ⇒ hasta la última (exclusivo)
	// Rebuild borra TODO lo del punto (alertas de los dos flujos, colores,
	// mensajes, observaciones, gestión y estados) y lo reconstruye. Nunca en production.
	Rebuild bool
	// DropManagement permite que Rebuild borre un punto con gestión
	// (clasificación, etiqueta u observaciones). Sin él, Rebuild aborta sin escribir.
	DropManagement bool
}

// BackfillResult es el resumen del backfill.
type BackfillResult struct {
	PointReplay
	Deleted int64
	Points  []PointReplay
}

// ReplayService recorre rule_evaluations y motion_evaluations (solo lectura)
// con EXACTAMENTE el mismo caso de uso que la evaluación en vivo:
//   - CatchUp (al arrancar y cada hora en el minuto 55): las horas posteriores
//     a la última procesada de cada flujo, publicando avisos.
//   - Backfill (cmd/alert_management -backfill): el histórico, sin publicar avisos.
//
// Los dos flujos se intercalan por hora: en cada evaluated_at, primero FUGA y
// después MOVIMIENTO (§7.2). Las horas de FUGA sin cálculo no tienen fila: el
// salto de evaluated_at se procesa como horas NO_CALCULABLE.
type ReplayService struct {
	processor *ProcessingService
	levels    ports.LevelSource
	motions   ports.MotionSource // nil ⇒ solo FUGA
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

// WithMotionSource agrega el flujo MOVIMIENTO al catch-up y al backfill.
func (s *ReplayService) WithMotionSource(m ports.MotionSource) *ReplayService {
	s.motions = m
	return s
}

// points son los puntos de los dos flujos, ascendentes y sin repetir.
func (s *ReplayService) points(ctx context.Context) ([]int64, error) {
	points, err := s.levels.ListPoints(ctx)
	if err != nil {
		s.log.Error("[ReplayService] Error listando los puntos de rule_evaluations", "error", err)
		return nil, domain.ErrConsultingRuleEvaluations
	}
	if s.motions != nil {
		more, err := s.motions.ListPoints(ctx)
		if err != nil {
			s.log.Error("[ReplayService] Error listando los puntos de motion_evaluations", "error", err)
			return nil, domain.ErrConsultingRuleEvaluations
		}
		points = append(points, more...)
	}
	slices.Sort(points)
	return slices.Compact(points), nil
}

// CatchUp procesa, publicando avisos, las horas que faltan de cada punto con
// historial. Un flujo sin estado se saltea (espera al backfill). Un error en
// un punto lo detiene (el orden importa) y no a los demás.
func (s *ReplayService) CatchUp(ctx context.Context) ([]PointReplay, error) {
	points, err := s.points(ctx)
	if err != nil {
		return nil, err
	}
	summary := make([]PointReplay, 0, len(points))
	for _, pointID := range points {
		if ctx.Err() != nil {
			break
		}
		leak, motion, err := s.lastEvaluatedAt(ctx, pointID)
		if err != nil {
			summary = append(summary, PointReplay{PointID: pointID, Err: err})
			continue
		}
		if leak == nil && motion == nil {
			summary = append(summary, PointReplay{PointID: pointID, WithoutHistory: true})
			continue
		}
		item := s.replayPoint(ctx, pointID, cursor{leak != nil, deref(leak)}, cursor{motion != nil, deref(motion)}, time.Time{}, entities.SourceCatchUp)
		if item.Hours > 0 {
			s.log.Warn("[ReplayService] Horas procesadas por el catch-up (evento perdido o no publicado)",
				"measuring_point_id", pointID, "hours", item.Hours, "motion_hours", item.MotionHours, "notifications", item.Notifications)
		}
		summary = append(summary, item)
	}
	return summary, nil
}

// Backfill reconstruye el histórico sin publicar avisos. Idempotente: retoma
// desde la última hora procesada de cada flujo. Un error aborta (repetirlo retoma).
func (s *ReplayService) Backfill(ctx context.Context, cmd BackfillCommand) (BackfillResult, error) {
	result := BackfillResult{}
	if cmd.DropManagement && !cmd.Rebuild {
		return result, domain.ErrDropManagementRequiresRebuild
	}
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
		if points, err = s.points(ctx); err != nil {
			return result, err
		}
	}

	// Guardas (§11.2): con -rebuild, ningún punto con una alerta vigente se
	// toca (ni con -drop-management), y sin -drop-management tampoco uno con
	// gestión. Se revisan todos antes de borrar el primero.
	counts := make(map[int64]ports.ResetCounts, len(points))
	if cmd.Rebuild {
		for _, pointID := range points {
			c, err := s.reset.CountPoint(ctx, pointID)
			if err != nil {
				s.log.Error("[ReplayService] Error contando lo que borraría el rebuild", "measuring_point_id", pointID, "error", err)
				return result, domain.ErrConsultingAlerts
			}
			counts[pointID] = c
			if c.Active > 0 {
				s.log.Error("[ReplayService] Rebuild abortado: el punto tiene una alerta vigente",
					"measuring_point_id", pointID, "active", c.Active)
				result.Points = append(result.Points, PointReplay{PointID: pointID, Reset: c})
				return result, domain.ErrActiveAlertPresent
			}
			if c.HasManagement() && !cmd.DropManagement {
				s.log.Error("[ReplayService] Rebuild abortado: el punto tiene gestión y no se pasó -drop-management",
					"measuring_point_id", pointID, "classified", c.Classified, "labeled", c.Labeled, "observations", c.Observations)
				result.Points = append(result.Points, PointReplay{PointID: pointID, Reset: c})
				return result, domain.ErrManagementPresent
			}
		}
	}

	for _, pointID := range points {
		var reset ports.ResetCounts
		if cmd.Rebuild {
			reset = counts[pointID]
			s.log.Warn("[ReplayService] Rebuild: se borra todo lo del punto",
				"measuring_point_id", pointID, "alerts", reset.Alerts, "messages", reset.Messages,
				"classified", reset.Classified, "labeled", reset.Labeled, "observations", reset.Observations)
			deleted, err := s.reset.ResetPoint(ctx, pointID)
			if err != nil {
				s.log.Error("[ReplayService] Error borrando para el rebuild", "measuring_point_id", pointID, "error", err)
				return result, domain.ErrPersistingAlerts
			}
			result.Deleted += deleted
		}

		from := cmd.From.Add(-time.Nanosecond)
		if cmd.From.IsZero() {
			from = time.Time{}
		}
		leak, motion, err := s.lastEvaluatedAt(ctx, pointID)
		if err != nil {
			return result, err
		}
		item := s.replayPoint(ctx, pointID, cursor{true, later(from, leak)}, cursor{true, later(from, motion)}, cmd.To.UTC(), entities.SourceBackfill)
		item.Reset = reset
		result.Points = append(result.Points, item)
		result.add(item)
		if item.Err != nil {
			return result, item.Err
		}
	}
	return result, nil
}

func later(t time.Time, last *time.Time) time.Time {
	if last != nil && last.After(t) {
		return *last
	}
	return t
}

func deref(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

// lastEvaluatedAt devuelve la última hora procesada de cada flujo (nil = sin estado).
func (s *ReplayService) lastEvaluatedAt(ctx context.Context, pointID int64) (leak, motion *time.Time, err error) {
	err = s.states.Transaction(ctx, func(tx ports.AlertTx) error {
		st, err := tx.GetState(ctx, pointID, entities.EventTypeLeak)
		if err != nil {
			return err
		}
		if st != nil {
			leak = st.LastEvaluatedAt
		}
		if st, err = tx.GetState(ctx, pointID, entities.EventTypeMovement); err != nil {
			return err
		}
		if st != nil {
			motion = st.LastEvaluatedAt
		}
		return nil
	})
	if err != nil {
		s.log.Error("[ReplayService] Error leyendo el estado de procesamiento", "measuring_point_id", pointID, "error", err)
		return nil, nil, domain.ErrConsultingAlerts
	}
	return leak, motion, nil
}

// cursor es desde dónde se lee un flujo (enabled = false ⇒ el flujo no se lee).
type cursor struct {
	enabled bool
	after   time.Time
}

// stream lee un flujo por lotes y entrega sus horas en orden.
type stream[T any] struct {
	buf   []T
	pos   int
	done  bool
	after time.Time
	fetch func(after time.Time) ([]T, error)
	at    func(T) time.Time
	limit int
}

func (st *stream[T]) peek() (*T, error) {
	if st.pos == len(st.buf) && !st.done {
		batch, err := st.fetch(st.after)
		if err != nil {
			return nil, err
		}
		st.buf, st.pos = batch, 0
		st.done = len(batch) < st.limit
		if len(batch) > 0 {
			st.after = st.at(batch[len(batch)-1])
		}
	}
	if st.pos < len(st.buf) {
		return &st.buf[st.pos], nil
	}
	return nil, nil
}

// replayPoint intercala los dos flujos del punto por hora (FUGA primero).
func (s *ReplayService) replayPoint(ctx context.Context, pointID int64, leakFrom, motionFrom cursor, to time.Time, source entities.Source) PointReplay {
	item := PointReplay{PointID: pointID}
	leak := &stream[entities.LevelNotice]{
		done: !leakFrom.enabled, after: leakFrom.after, limit: s.cfg.BatchSize,
		at: func(n entities.LevelNotice) time.Time { return n.EvaluatedAt },
		fetch: func(after time.Time) ([]entities.LevelNotice, error) {
			return s.levels.ListAfter(ctx, pointID, after, to, s.cfg.BatchSize)
		},
	}
	motion := &stream[entities.MotionNotice]{
		done: !motionFrom.enabled || s.motions == nil, after: motionFrom.after, limit: s.cfg.BatchSize,
		at: func(n entities.MotionNotice) time.Time { return n.EvaluatedAt },
		fetch: func(after time.Time) ([]entities.MotionNotice, error) {
			return s.motions.ListAfter(ctx, pointID, after, to, s.cfg.BatchSize)
		},
	}
	for {
		if err := ctx.Err(); err != nil {
			item.Err = err
			return item
		}
		l, err := leak.peek()
		if err != nil {
			s.log.Error("[ReplayService] Error leyendo rule_evaluations", "measuring_point_id", pointID, "error", err)
			item.Err = domain.ErrConsultingRuleEvaluations
			return item
		}
		m, err := motion.peek()
		if err != nil {
			s.log.Error("[ReplayService] Error leyendo motion_evaluations", "measuring_point_id", pointID, "error", err)
			item.Err = domain.ErrConsultingRuleEvaluations
			return item
		}
		var r ProcessResult
		switch {
		case l == nil && m == nil:
			return item
		case l != nil && (m == nil || !l.EvaluatedAt.After(m.EvaluatedAt)):
			r, err = s.processor.Process(ctx, *l, source)
			leak.pos++
		default:
			r, err = s.processor.ProcessMotion(ctx, *m, source)
			motion.pos++
			item.MotionHours += r.Hours
		}
		if err != nil {
			item.Err = err
			return item
		}
		item.record(r)
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
	p.MotionHours += o.MotionHours
	p.Messages += o.Messages
	p.Notifications += o.Notifications
	p.Opened += o.Opened
	p.Reactivated += o.Reactivated
	p.Finished += o.Finished
}
