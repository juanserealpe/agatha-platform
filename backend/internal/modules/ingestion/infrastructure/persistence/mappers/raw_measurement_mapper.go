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
		MeasuringPointID: entity.MeasuringPointID,
		MetricID:         entity.MetricID,
		Timestamp:        entity.Timestamp,
		Value:            entity.Value,
		Source:           entity.Source,
		IngestionBatchID: entity.IngestionBatchID,
	}
}
