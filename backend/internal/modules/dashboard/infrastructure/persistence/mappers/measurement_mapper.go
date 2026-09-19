package mappers

import (
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/infrastructure/persistence/models"
)

func ToRawMeasurement(
	model models.RawMeasurementModel,
) entities.RawMeasurement {

	return entities.RawMeasurement{
		MetricID:   model.MetricID,
		Timestamp:  model.Timestamp,
		MetricName: model.MetricName,
		Unit:       model.Unit,
		Value:      model.Value,
	}
}
