package mappers

import (
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/persistence/models"
)

func ToDeviceModel(entity *entities.Device) *models.DeviceModel {
	return &models.DeviceModel{
		ID:               entity.ID,
		MeasuringPointID: entity.MeasuringPointID,
		Name:             entity.Name,
		Active:           entity.Active,
		CreatedAt:        entity.CreatedAt,
		UpdatedAt:        entity.UpdatedAt,
	}
}

func ToDeviceEntity(model *models.DeviceModel) *entities.Device {
	return &entities.Device{
		ID:               model.ID,
		MeasuringPointID: model.MeasuringPointID,
		Name:             model.Name,
		Active:           model.Active,
		CreatedAt:        model.CreatedAt,
		UpdatedAt:        model.UpdatedAt,
	}
}
