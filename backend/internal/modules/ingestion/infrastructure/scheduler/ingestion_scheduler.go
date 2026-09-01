package scheduler

import (
	"context"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/application/services"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

type IngestionScheduler struct {
	service *services.IngestService
	logger  *logger.Logger
}

func (s *IngestionScheduler) Start(ctx context.Context) {

	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	for {
		select {

		case <-ticker.C:
			s.run(ctx)

		case <-ctx.Done():
			return
		}
	}
}

func (s *IngestionScheduler) run(ctx context.Context) {

	now := time.Now()

	start := now.Add(-15 * time.Minute)
	end := now

	err := s.service.IngestDataOs(
		ctx,
		11556,
		start,
		end,
	)

	if err != nil {
		// log error
		s.logger.Error("error al ingestar datos", "error", err)
	}
}
