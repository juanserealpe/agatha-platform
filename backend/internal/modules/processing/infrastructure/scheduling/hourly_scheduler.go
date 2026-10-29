// Package scheduling contiene el reconciliador horario de Processing.
package scheduling

import (
	"context"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// runTimeout es el tiempo máximo de una reconciliación.
const runTimeout = 10 * time.Minute

// Reconciler es lo que el scheduler necesita del caso de uso UC-P2.
type Reconciler interface {
	Reconcile(ctx context.Context, now time.Time) ([]services.PointReconciliation, error)
}

// HourlyScheduler ejecuta Reconcile una vez al arrancar y luego cada hora en
// el minuto tickMinute (UTC). Es la red de seguridad que garantiza que
// ninguna hora quede sin fila: no contiene reglas.
type HourlyScheduler struct {
	reconciler Reconciler
	log        *logger.Logger
	tickMinute int
	now        func() time.Time
}

func NewHourlyScheduler(reconciler Reconciler, log *logger.Logger, tickMinute int) *HourlyScheduler {
	if tickMinute < 0 || tickMinute > 59 {
		tickMinute = 35
	}

	return &HourlyScheduler{
		reconciler: reconciler,
		log:        log,
		tickMinute: tickMinute,
		now:        time.Now,
	}
}

// Start bloquea hasta que ctx se cancela.
func (s *HourlyScheduler) Start(ctx context.Context) {
	s.run(ctx)

	for {
		timer := time.NewTimer(time.Until(s.nextRun(s.now())))

		select {
		case <-timer.C:
			s.run(ctx)
		case <-ctx.Done():
			timer.Stop()
			return
		}
	}
}

// nextRun devuelve el próximo instante HH:tickMinute estrictamente posterior a now.
func (s *HourlyScheduler) nextRun(now time.Time) time.Time {
	next := now.UTC().Truncate(time.Hour).Add(time.Duration(s.tickMinute) * time.Minute)
	if !next.After(now) {
		next = next.Add(time.Hour)
	}
	return next
}

func (s *HourlyScheduler) run(parent context.Context) {
	if parent.Err() != nil {
		return
	}

	ctx, cancel := context.WithTimeout(parent, runTimeout)
	defer cancel()

	summary, err := s.reconciler.Reconcile(ctx, s.now().UTC())
	if err != nil {
		s.log.Error("[HourlyScheduler] Falló la reconciliación", "error", err)
		return
	}

	var hours, inserted, failed int
	for _, item := range summary {
		hours += item.Result.Planned
		inserted += item.Result.Inserted
		if item.Err != nil {
			failed++
		}
	}

	if hours > 0 || failed > 0 {
		s.log.Info("[HourlyScheduler] Reconciliación completada",
			"points", len(summary), "hours", hours, "inserted", inserted, "failed", failed)
	}
}
