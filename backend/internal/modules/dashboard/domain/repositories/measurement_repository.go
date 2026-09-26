package repositories

import (
	"context"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/domain/entities"
)

type MeasurementRepository interface {
	GetMeasurements(ctx context.Context, deviceID int64, startDate time.Time, endDate time.Time,
	) ([]entities.RawMeasurement, error)
}
