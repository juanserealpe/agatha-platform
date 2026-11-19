package scheduling

import (
	"context"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// DefaultMotionMinute es el minuto (UTC) por defecto de RULE_ENGINE_MOTION_MINUTE.
const DefaultMotionMinute = 40

// MotionRunner es lo que el scheduler necesita del caso de uso.
type MotionRunner interface {
	RunLive(ctx context.Context) ([]services.MotionPointRun, error)
}

// MotionScheduler evalúa el movimiento al arrancar y cada hora en el minuto
// tickMinute (UTC). No depende de Processing. No contiene reglas.
type MotionScheduler struct {
	runner     MotionRunner
	log        *logger.Logger
	tickMinute int
	now        func() time.Time
}

func NewMotionScheduler(runner MotionRunner, log *logger.Logger, tickMinute int) *MotionScheduler {
	if tickMinute < 0 || tickMinute > 59 {
		tickMinute = DefaultMotionMinute
	}
	return &MotionScheduler{runner: runner, log: log, tickMinute: tickMinute, now: time.Now}
}

// Start bloquea hasta que ctx se cancela.
func (s *MotionScheduler) Start(ctx context.Context) {
	s.run(ctx)
	for {
		timer := time.NewTimer(time.Until(nextTick(s.now(), s.tickMinute)))
		select {
		case <-timer.C:
			s.run(ctx)
		case <-ctx.Done():
			timer.Stop()
			return
		}
	}
}

// nextTick devuelve el próximo HH:minute estrictamente posterior a now.
func nextTick(now time.Time, minute int) time.Time {
	next := now.UTC().Truncate(time.Hour).Add(time.Duration(minute) * time.Minute)
	if !next.After(now) {
		next = next.Add(time.Hour)
	}
	return next
}

func (s *MotionScheduler) run(parent context.Context) {
	if parent.Err() != nil {
		return
	}
	ctx, cancel := context.WithTimeout(parent, runTimeout)
	defer cancel()

	runs, err := s.runner.RunLive(ctx)
	if err != nil {
		s.log.Error("[MotionScheduler] Falló la evaluación de movimiento", "error", err)
		return
	}
	var saved, published, failed int
	var withoutHistory []int64
	for _, r := range runs {
		saved += r.Saved
		published += r.Published
		if r.Err != nil {
			failed++
			s.log.Error("[MotionScheduler] Falló la evaluación de movimiento del punto", "measuring_point_id", r.PointID, "error", r.Err)
		}
		if r.WithoutHistory {
			withoutHistory = append(withoutHistory, r.PointID)
		}
	}
	if saved > 0 || failed > 0 {
		s.log.Info("[MotionScheduler] Movimiento evaluado", "points", len(runs), "hours", saved, "published", published, "failed", failed)
	}
	if len(withoutHistory) > 0 {
		s.log.Warn("[MotionScheduler] Puntos sin motion_evaluations: correr rule_engine -backfill -motion", "points", withoutHistory)
	}
}
