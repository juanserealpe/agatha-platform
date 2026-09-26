package ports

import (
	"context"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
)

type DataProvider interface {
	GetMeasurements(ctx context.Context, Device *entities.Device, start time.Time, end time.Time,
	) ([]entities.RawMeasurement, error)
}
