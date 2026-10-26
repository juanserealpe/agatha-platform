// Package scheduling contiene el proceso de fondo de Alert Management: el
// catch-up horario.
package scheduling

import (
	"context"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// DefaultCatchUpMinute: después del catch-up del Rule Engine (minuto 50).
const DefaultCatchUpMinute = 55

const catchUpTimeout = 10 * time.Minute

// CatchUpper es lo que necesita el scheduler del caso de uso.
type CatchUpper interface {
	CatchUp(ctx context.Context) ([]services.PointReplay, error)
}

// CatchUpScheduler ejecuta CatchUp al arrancar y cada hora en el minuto tickMinute (UTC).
type CatchUpScheduler struct {
	catchUp    CatchUpper
	log        *logger.Logger
	tickMinute int
	now        func() time.Time
}

func NewCatchUpScheduler(catchUp CatchUpper, log *logger.Logger, tickMinute int) *CatchUpScheduler {
	if tickMinute < 0 || tickMinute > 59 {
		tickMinute = DefaultCatchUpMinute
	}
	return &CatchUpScheduler{catchUp: catchUp, log: log, tickMinute: tickMinute, now: time.Now}
}

// Start bloquea hasta que ctx se cancela.
func (s *CatchUpScheduler) Start(ctx context.Context) {
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

func (s *CatchUpScheduler) nextRun(now time.Time) time.Time {
	next := now.UTC().Truncate(time.Hour).Add(time.Duration(s.tickMinute) * time.Minute)
	if !next.After(now) {
		next = next.Add(time.Hour)
	}
	return next
}

func (s *CatchUpScheduler) run(parent context.Context) {
	if parent.Err() != nil {
		return
	}
	ctx, cancel := context.WithTimeout(parent, catchUpTimeout)
	defer cancel()

	summary, err := s.catchUp.CatchUp(ctx)
	if err != nil {
		s.log.Error("[CatchUpScheduler] Falló el catch-up", "error", err)
		return
	}
	var hours, notifications, failed, withoutHistory int
	for _, p := range summary {
		hours += p.Hours
		notifications += p.Notifications
		if p.Err != nil {
			failed++
		}
		if p.WithoutHistory {
			withoutHistory++
		}
	}
	if hours > 0 || failed > 0 {
		s.log.Info("[CatchUpScheduler] Catch-up completado", "points", len(summary), "hours", hours,
			"notifications", notifications, "failed", failed)
	}
	if withoutHistory > 0 {
		s.log.Warn("[CatchUpScheduler] Puntos sin historial de alertas: correr alert_management -backfill", "points", withoutHistory)
	}
}
