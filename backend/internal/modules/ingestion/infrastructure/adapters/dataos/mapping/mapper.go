package mapping

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/adapters/dataos/dashboard"
)

type Mapper struct {
	timestampParser *TimestampParser
}

func NewMapper(
	timestampParser *TimestampParser,
) *Mapper {
	return &Mapper{
		timestampParser: timestampParser,
	}
}

func (m *Mapper) MapSeries(
	device *entities.Device,
	series dashboard.Series,
	metricKey string,
	metricID uint64,
) ([]entities.RawMeasurement, error) {

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
			MetricID:    metricID,
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

	return measurements, nil
}
