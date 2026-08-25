package mappers

import (
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/persistence/models"
)

func ToRawMeasurementModel(
	entity *entities.RawMeasurement,
) *models.RawMeasurementModel {
	if entity == nil {
		return nil
	}

	return &models.RawMeasurementModel{
		ID:        entity.ID,
		DeviceID:  entity.DeviceID,
		MetricID:  entity.MetricID,
		Timestamp: entity.Timestamp,
		Value:     entity.Value,
		Source:    entity.Source,
	}
}

func ToRawMeasurementEntity(
	model *models.RawMeasurementModel,
) *entities.RawMeasurement {
	if model == nil {
		return nil
	}

	return &entities.RawMeasurement{
		ID:        model.ID,
		DeviceID:  model.DeviceID,
		MetricID:  model.MetricID,
		Timestamp: model.Timestamp,
		Value:     model.Value,
		Source:    model.Source,
	}
}
