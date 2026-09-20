package repositories

import (
	"context"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/domain/entities"
)

type DeviceRepository interface {
	GetAllDeviceSummaries(ctx context.Context) ([]entities.DeviceSummary, error)
	ExistsDevice(ctx context.Context, deviceID int64) (bool, error)
}
