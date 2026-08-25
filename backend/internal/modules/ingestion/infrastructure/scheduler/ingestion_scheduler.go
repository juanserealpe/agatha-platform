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

		case <-ctx.Done():
			return
		}
	}
}
