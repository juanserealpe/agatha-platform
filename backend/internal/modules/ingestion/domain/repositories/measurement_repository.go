package repositories

import (
	"context"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
)

type MeasurementRepository interface {
	Save(ctx context.Context, measurement entities.RawMeasurement) error

	SaveBatch(
		ctx context.Context,
		measurements []entities.RawMeasurement,
	) error

	FindLastTimestamp(
		ctx context.Context,
		deviceID string,
	) (*time.Time, error)

	Exists(
		ctx context.Context,
		deviceID string,
		metricID string,
		timestamp time.Time,
	) (bool, error)
}
