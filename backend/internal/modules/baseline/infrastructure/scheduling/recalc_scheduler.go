package scheduling

import (
	"context"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// runTimeout es el tiempo máximo de una ejecución de RecalculateDue.
const runTimeout = 10 * time.Minute

// DueRecalculator es lo que el scheduler necesita del caso de uso UC-B2.
type DueRecalculator interface {
	RecalculateDue(ctx context.Context, now time.Time) ([]services.PointRecalculation, error)
}

// RecalcScheduler ejecuta RecalculateDue una vez al arrancar y luego cada hora
// en el minuto tickMinute (UTC). No contiene reglas: decide RecalculationPolicy.
//
// Tick horario y no cron semanal: si el proceso estaba caído a la hora del
// cron se perdería la semana; con la política basada en created_at el
// recálculo ocurre en el primer tick disponible.
type RecalcScheduler struct {
	recalculator DueRecalculator
	log          *logger.Logger
	tickMinute   int
	now          func() time.Time
}

func NewRecalcScheduler(recalculator DueRecalculator, log *logger.Logger, tickMinute int) *RecalcScheduler {
	if tickMinute < 0 || tickMinute > 59 {
		tickMinute = 10
	}

	return &RecalcScheduler{
		recalculator: recalculator,
		log:          log,
		tickMinute:   tickMinute,
		now:          time.Now,
	}
}

// Start bloquea hasta que ctx se cancela.
func (s *RecalcScheduler) Start(ctx context.Context) {
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
func (s *RecalcScheduler) nextRun(now time.Time) time.Time {
	next := now.UTC().Truncate(time.Hour).Add(time.Duration(s.tickMinute) * time.Minute)
	if !next.After(now) {
		next = next.Add(time.Hour)
	}
	return next
}

func (s *RecalcScheduler) run(parent context.Context) {
	if parent.Err() != nil {
		return
	}

	ctx, cancel := context.WithTimeout(parent, runTimeout)
	defer cancel()

	now := s.now().UTC()
	summary, err := s.recalculator.RecalculateDue(ctx, now)
	if err != nil {
		s.log.Error("[RecalcScheduler] Falló el recálculo programado", "error", err)
		return
	}

	var due, created, failed int
	for _, item := range summary {
		if item.Due {
			due++
		}
		if item.Result != nil && item.Result.Created {
			created++
		}
		if item.Err != nil {
			failed++
		}
	}

	if due > 0 || failed > 0 {
		s.log.Info("[RecalcScheduler] Recálculo programado completado",
			"points", len(summary), "due", due, "created", created, "failed", failed)
	}
}
