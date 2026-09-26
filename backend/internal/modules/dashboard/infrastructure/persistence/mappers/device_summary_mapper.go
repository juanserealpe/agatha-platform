package mappers

import (
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/infrastructure/persistence/models"
)

func ToDeviceSummary(model models.DeviceSummaryModel) entities.DeviceSummary {
	return entities.DeviceSummary{
		ID:               model.MeasuringPointID,
		Name:             model.Name,
		LastConnection:   model.LastConnection,
		LastBatteryLevel: model.LastBatteryLevel,
	}
}
