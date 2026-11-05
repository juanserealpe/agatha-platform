package repositories

import (
	"context"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
)

type DeviceRepository interface {
	// FindByMeasuringPointID devuelve nil, nil si el punto no existe.
	FindByMeasuringPointID(ctx context.Context, measuringPointID int64) (*entities.Device, error)
}
