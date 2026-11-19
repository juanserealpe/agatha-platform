package services

import (
	"context"
	"strings"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
)

// RunLive es la pasada del MotionScheduler: da la v1 de punto nuevo a los
// puntos de devices que no tienen versión y evalúa, publicando, las horas
// pendientes de cada punto con versión. Un error en un punto no detiene a los
// demás; solo falla si no se pueden listar los puntos.
func (s *MotionEvaluationService) RunLive(ctx context.Context) ([]MotionPointRun, error) {
	devices, err := s.source.ListPoints(ctx)
	if err != nil {
		s.log.Error("[MotionEvaluationService] Error listando los puntos de devices", "error", err)
		return nil, domain.ErrConsultingRawMeasurements
	}
	known, err := s.thresholds.ListPoints(ctx)
	if err != nil {
		s.log.Error("[MotionEvaluationService] Error listando los puntos con umbrales de movimiento", "error", err)
		return nil, domain.ErrConsultingThresholds
	}
	has := make(map[int64]bool, len(known))
	for _, p := range known {
		has[p] = true
	}
	for _, p := range devices {
		if !has[p] && s.thresholdUCs != nil {
			if _, err := s.thresholdUCs.EnsureInitialVersion(ctx, p); err == nil {
				known = append(known, p)
			}
		}
	}

	runs := make([]MotionPointRun, 0, len(known))
	for _, p := range known {
		if ctx.Err() != nil {
			s.log.Warn("[MotionEvaluationService] Pasada de movimiento interrumpida", "error", ctx.Err())
			break
		}
		runs = append(runs, s.EvaluateLive(ctx, p))
	}
	return runs, nil
}

// MotionBackfillConfig son los parámetros del backfill de movimiento.
type MotionBackfillConfig struct {
	// Environment es AGATHA_ENV: con "production" no se permite Rebuild.
	Environment string
}

// MotionBackfillCommand es la entrada de rule_engine -backfill -motion.
type MotionBackfillCommand struct {
	PointID int64     // 0 ⇒ todos los puntos con versión de movimiento
	From    time.Time // cero ⇒ desde la primera medición (solo aplica sin filas previas o con Rebuild)
	To      time.Time // cero ⇒ hasta el inicio de la hora actual; fin exclusivo
	// Rebuild borra motion_evaluations de [From, To) del punto y recalcula.
	Rebuild bool
}

// MotionBackfillResult es el resumen del backfill.
type MotionBackfillResult struct {
	Deleted int64
	Points  []MotionPointRun
}

// MotionBackfillService carga el histórico de motion_evaluations con
// EXACTAMENTE la misma evaluación que en vivo, en orden y por punto, con
// trigger BACKFILL y sin publicar. Es idempotente y retoma: sigue desde la
// última fila del punto (la hora siguiente necesita el estado de la anterior).
type MotionBackfillService struct {
	evaluator *MotionEvaluationService
	cfg       MotionBackfillConfig
}

func NewMotionBackfillService(evaluator *MotionEvaluationService, cfg MotionBackfillConfig) *MotionBackfillService {
	return &MotionBackfillService{evaluator: evaluator, cfg: cfg}
}

// Backfill recorre los puntos. Un error detiene el punto y se devuelve el
// primero; lo guardado queda (repetirlo retoma).
func (b *MotionBackfillService) Backfill(ctx context.Context, cmd MotionBackfillCommand) (MotionBackfillResult, error) {
	var result MotionBackfillResult
	s := b.evaluator
	if cmd.PointID < 0 {
		return result, domain.ErrInvalidMeasuringPointID
	}
	if !cmd.From.IsZero() && !cmd.To.IsZero() && !cmd.From.Before(cmd.To) {
		return result, domain.ErrInvalidTimeRange
	}
	if cmd.Rebuild && strings.EqualFold(strings.TrimSpace(b.cfg.Environment), ProductionEnvironment) {
		return result, domain.ErrRebuildForbidden
	}

	points := []int64{cmd.PointID}
	if cmd.PointID == 0 {
		var err error
		if points, err = s.thresholds.ListPoints(ctx); err != nil {
			s.log.Error("[MotionBackfillService] Error listando los puntos con umbrales de movimiento", "error", err)
			return result, domain.ErrConsultingThresholds
		}
	}

	var firstErr error
	for _, pointID := range points {
		run, deleted := b.backfillPoint(ctx, pointID, cmd)
		result.Deleted += deleted
		result.Points = append(result.Points, run)
		if run.Err != nil && firstErr == nil {
			firstErr = run.Err
		}
		if ctx.Err() != nil {
			break
		}
	}
	return result, firstErr
}

func (b *MotionBackfillService) backfillPoint(ctx context.Context, pointID int64, cmd MotionBackfillCommand) (MotionPointRun, int64) {
	s := b.evaluator
	unlock := s.lock(pointID)
	defer unlock()

	run := MotionPointRun{PointID: pointID}
	var deleted int64
	if cmd.Rebuild {
		n, err := s.evaluations.DeleteRange(ctx, pointID, cmd.From, cmd.To)
		if err != nil {
			s.log.Error("[MotionBackfillService] Error borrando motion_evaluations", "measuring_point_id", pointID, "error", err)
			run.Err = domain.ErrPersistingMotionEvaluations
			return run, 0
		}
		deleted = n
		if !cmd.To.IsZero() {
			if later, err := s.evaluations.CountAfter(ctx, pointID, cmd.To); err == nil && later > 0 {
				s.log.Warn("[MotionBackfillService] Quedan filas posteriores a -to calculadas con el estado anterior: conviene reconstruir hasta el final",
					"measuring_point_id", pointID, "to", cmd.To, "rows", later)
			}
		}
	}

	prev, err := s.evaluations.GetLastBefore(ctx, pointID, cmd.To)
	if err != nil {
		s.log.Error("[MotionBackfillService] Error consultando la última hora evaluada", "measuring_point_id", pointID, "error", err)
		run.Err = domain.ErrConsultingMotionEvaluations
		return run, deleted
	}

	var start time.Time
	if prev != nil {
		start = prev.EvaluatedAt.Add(time.Hour)
	} else {
		first, err := s.source.FirstMeasurementAt(ctx, pointID)
		if err != nil {
			s.log.Error("[MotionBackfillService] Error consultando la primera medición", "measuring_point_id", pointID, "error", err)
			run.Err = domain.ErrConsultingRawMeasurements
			return run, deleted
		}
		if first == nil {
			run.WithoutMeasurements = true
			return run, deleted
		}
		start = ceilHour(*first)
		if from := ceilHour(cmd.From); !cmd.From.IsZero() && from.After(start) {
			start = from
		}
	}

	end := s.clock.Now().UTC().Truncate(time.Hour)
	if !cmd.To.IsZero() {
		end = ceilHour(cmd.To).Add(-time.Hour) // t < To
	}
	if start.After(end) {
		return run, deleted
	}

	backfill := func(time.Time) entities.MotionTrigger { return entities.MotionTriggerBackfill }
	s.evaluateRange(ctx, &run, prev, start, end, backfill, false)
	s.log.Info("[MotionBackfillService] Backfill de movimiento del punto",
		"measuring_point_id", pointID, "hours", run.Hours, "saved", run.Saved, "duplicates", run.Duplicates,
		"no_calculable", run.NoCalculable, "no_thresholds", run.NoThresholds, "opened", run.Opened, "cleared", run.Cleared)
	return run, deleted
}
