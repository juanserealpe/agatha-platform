package ports

import (
	"context"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/repositories"
)

type IngestionUnitOfWork interface {
	Execute(
		ctx context.Context,
		fn func(
			ctx context.Context,
			measurementRepository repositories.MeasurementRepository,
			ingestionStateRepository repositories.IngestionStateRepository,
		) error,
	) error
}
