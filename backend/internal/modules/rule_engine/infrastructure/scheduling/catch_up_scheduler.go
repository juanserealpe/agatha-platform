// Package scheduling contiene el catch-up horario del Rule Engine.
package scheduling

import (
	"context"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// DefaultCatchUpMinute es el minuto (UTC) por defecto: después de la
// evaluación horaria de Processing (minuto 35), para que sus eventos ya se
// hayan consumido y el catch-up solo vea las horas sin evento.
const DefaultCatchUpMinute = 50

// runTimeout es el tiempo máximo de una pasada.
const runTimeout = 10 * time.Minute

// CatchUpper es lo que el scheduler necesita del caso de uso.
type CatchUpper interface {
	CatchUp(ctx context.Context) ([]services.PointCatchUp, error)
}

// CatchUpScheduler ejecuta CatchUp una vez al arrancar (cubre la caída) y
// luego cada hora en el minuto tickMinute (UTC). No contiene reglas.
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

// nextRun devuelve el próximo instante HH:tickMinute estrictamente posterior a now.
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

	ctx, cancel := context.WithTimeout(parent, runTimeout)
	defer cancel()

	summary, err := s.catchUp.CatchUp(ctx)
	if err != nil {
		s.log.Error("[CatchUpScheduler] Falló el catch-up", "error", err)
		return
	}

	var hours, published, failed, withoutHistory int
	for _, item := range summary {
		hours += item.Hours
		published += item.Published
		if item.Err != nil {
			failed++
		}
		if item.WithoutHistory {
			withoutHistory++
		}
	}

	if hours > 0 || failed > 0 {
		s.log.Info("[CatchUpScheduler] Catch-up completado",
			"points", len(summary), "hours", hours, "published", published, "failed", failed)
	}
	if withoutHistory > 0 {
		s.log.Warn("[CatchUpScheduler] Puntos sin ninguna rule_evaluation: correr rule_engine -backfill",
			"points", withoutHistory)
	}
}
