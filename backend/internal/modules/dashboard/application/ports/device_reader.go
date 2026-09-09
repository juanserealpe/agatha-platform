package ports

import (
	"context"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/domain/entities"
)

type DeviceReader interface {
	GetAllDeviceSummaries(ctx context.Context) ([]entities.DeviceSummary, error)
}
