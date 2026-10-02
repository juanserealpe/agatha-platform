package mappers

import (
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/infrastructure/persistence/models"
)

func ToRawMeasurement(
	model models.LatestMeasurementModel,
) entities.RawMeasurement {

	return entities.RawMeasurement{
		Timestamp:        model.Timestamp,
		MeasuringPointID: model.MeasuringPointID,
		Value:            model.Value,
		Source:           model.Source,
		Metric: entities.MetricDefinition{
			ID:   model.MetricID,
			Key:  model.MetricKey,
			Name: model.MetricName,
			Unit: model.Unit,
		},
	}
}
