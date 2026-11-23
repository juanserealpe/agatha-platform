package scheduler

import (
	"context"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// CatchUpRunner es CatchUpService.Run.
type CatchUpRunner interface {
	Run(ctx context.Context, now time.Time) ([]services.CatchUpResult, error)
}

// IngestionScheduler corre el catch-up de DataOS: una vez al arrancar
// (RunOnce, que el bootstrap ejecuta ANTES de los demás schedulers) y después
// en cada borde de interval + delay (Start).
type IngestionScheduler struct {
	service  CatchUpRunner
	logger   *logger.Logger
	interval time.Duration
	location *time.Location
	delay    time.Duration
	now      func() time.Time
}

func NewIngestionScheduler(
	service CatchUpRunner,
	logger *logger.Logger,
	interval time.Duration,
	location *time.Location,
	delay time.Duration,
) *IngestionScheduler {
	return &IngestionScheduler{
		service:  service,
		logger:   logger,
		interval: interval,
		location: location,
		delay:    delay,
		now:      time.Now,
	}
}

// RunOnce ejecuta una corrida completa y bloquea hasta terminarla.
func (s *IngestionScheduler) RunOnce(ctx context.Context) {
	started := s.now()
	results, err := s.service.Run(ctx, started)
	if err != nil {
		s.logger.Error("el catch-up de ingesta terminó con error", "error", err)
		return
	}

	requests, failed := 0, 0
	for _, r := range results {
		requests += r.Requests
		failed += r.Failed
	}
	s.logger.Info("catch-up de ingesta terminado", "puntos", len(results), "consultas", requests, "fallidas", failed,
		"duracion", s.now().Sub(started).Round(time.Millisecond))
}

// Start bloquea hasta que ctx se cancela. No corre al arrancar: eso es RunOnce.
func (s *IngestionScheduler) Start(ctx context.Context) {
	for {
		timer := time.NewTimer(time.Until(s.nextRun(s.now().In(s.location))))

		select {
		case <-timer.C:
			s.RunOnce(ctx)
		case <-ctx.Done():
			timer.Stop()
			return
		}
	}
}

func (s *IngestionScheduler) nextRun(now time.Time) time.Time {
	return now.Truncate(s.interval).Add(s.interval).Add(s.delay)
}
