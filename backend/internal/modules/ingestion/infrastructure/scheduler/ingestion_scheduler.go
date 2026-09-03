package scheduler

import (
	"context"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

type IngestionScheduler struct {
	service          *services.IngestService
	logger           *logger.Logger
	measuringPointID int64
	interval         time.Duration
	location         *time.Location
	delay            time.Duration
}

func NewIngestionScheduler(
	service *services.IngestService,
	logger *logger.Logger,
	measuringPointID int64,
	interval time.Duration,
	location *time.Location,
	delay time.Duration,
) *IngestionScheduler {
	return &IngestionScheduler{
		service:          service,
		logger:           logger,
		measuringPointID: measuringPointID,
		interval:         interval,
		location:         location,
		delay:            delay,
	}
}

func (s *IngestionScheduler) Start(ctx context.Context) {
	for {
		now := time.Now().In(s.location)
		nextRun := s.getNextRun(now)

		timer := time.NewTimer(time.Until(nextRun))

		select {
		case <-timer.C:
			s.run(ctx, nextRun)

		case <-ctx.Done():
			timer.Stop()
			return
		}
	}
}

func (s *IngestionScheduler) getNextRun(now time.Time) time.Time {
	nextBoundary := now.Truncate(s.interval).Add(s.interval)
	return nextBoundary.Add(s.delay)
}

func (s *IngestionScheduler) run(ctx context.Context, executionTime time.Time) {
	end := executionTime.Add(-s.delay)
	start := end.Add(-s.interval)

	s.logger.Info(
		"iniciando ingesta programada",
		"start", start,
		"end", end,
		"execution_time", executionTime,
	)

	if err := s.service.IngestDataOs(ctx, s.measuringPointID, start, end); err != nil {
		s.logger.Error("error durante la ingesta programada", "error", err)
		return
	}

	s.logger.Info("ingesta programada completada correctamente", "start", start, "end", end)
}