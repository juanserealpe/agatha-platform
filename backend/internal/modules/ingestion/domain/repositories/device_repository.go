package repositories

import (
	"context"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
)

type DeviceRepository interface {
	Save(ctx context.Context, device entities.Device) error
	FindByMeasuringPointID(ctx context.Context, measuringPointID int64) (*entities.Device, error)
	List(ctx context.Context) ([]entities.Device, error)
}
