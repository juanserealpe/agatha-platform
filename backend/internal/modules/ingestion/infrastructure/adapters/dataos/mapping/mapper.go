package mapping

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/adapters/dataos/dashboard"
)

type MappedMetric struct {
	Metric       entities.MetricDefinition
	Measurements []entities.RawMeasurement
}

type Mapper struct {
	catalog         *MetricCatalog
	timestampParser *TimestampParser
}

func NewMapper(
	catalog *MetricCatalog,
	timestampParser *TimestampParser,
) *Mapper {
	return &Mapper{
		catalog:         catalog,
		timestampParser: timestampParser,
	}

}

func (m *Mapper) MapSeries(
	device *entities.Device,
	series dashboard.Series,
) (*MappedMetric, error) {

	definition, ok := m.catalog.FindNormalized(series.Name)

	if !ok {
		return nil, fmt.Errorf(
			"unknown DataOS metric: %q",
			series.Name,
		)
	}

	now := time.Now()

	metric := entities.MetricDefinition{
		ID:        uuid.NewString(),
		DeviceID:  device.ID,
		Key:       definition.Key,
		Name:      definition.Name,
		Unit:      series.Unit,
		Category:  definition.Category,
		Active:    true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	measurements := make(
		[]entities.RawMeasurement,
		0,
		len(series.DataPoints),
	)

	for _, point := range series.DataPoints {

		timestamp, err := m.timestampParser.Parse(
			point.Timestamp,
		)

		if err != nil {
			return nil, fmt.Errorf(
				"parse timestamp %q for metric %q: %w",
				point.Timestamp,
				series.Name,
				err,
			)
		}

		measurement := entities.RawMeasurement{
			ID:          uuid.NewString(),
			DeviceID:    device.ID,
			MetricID:    metric.ID,
			Timestamp:   timestamp,
			Value:       point.Value,
			Source:      "dataos",
			ExternalKey: series.Name,
		}

		measurements = append(
			measurements,
			measurement,
		)
	}

	return &MappedMetric{
		Metric:       metric,
		Measurements: measurements,
	}, nil
}
