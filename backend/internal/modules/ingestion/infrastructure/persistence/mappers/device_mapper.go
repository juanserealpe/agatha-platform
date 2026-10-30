package mappers

import (
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/persistence/models"
)

func ToDeviceEntity(model *models.DeviceModel) *entities.Device {
	return &entities.Device{
		MeasuringPointID: model.MeasuringPointID,
		Name:             model.Name,
		Active:           model.Active,
		CreatedAt:        model.CreatedAt,
		UpdatedAt:        model.UpdatedAt,
	}
}
