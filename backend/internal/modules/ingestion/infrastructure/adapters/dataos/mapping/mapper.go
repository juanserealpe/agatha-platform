package mapping

import (
	"fmt"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/adapters/dataos/dashboard"
)

var metricIDMap = map[string]uint64{
	"Pitch":          1,
	"Roll":           2,
	"Voltaje (mV)":   3,
	"Corriente (mA)": 4,
	"Batería (%)":    5,
	"RMS (ADC)":      6,

	"Actividad":             7,
	"Mag promedio (g)":      8,
	"Act. pico (g/s)":       9,
	"Razon R (fuga/viento)": 10,

	"B0 viento 30-300Hz":  11,
	"B1 fuga 500-1200Hz":  12,
	"B2 fuga 1200-2000Hz": 13,
	"B3 guarda 2-2.4kHz":  14,

	"Pitch base":   15,
	"Roll base":    16,
	"RMS acustico": 17,

	"Planitud espectral": 18,
	"Deriva (grados)":    19,
}

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

func (m *Mapper) MapSeries(device *entities.Device, series dashboard.Series,
) ([]entities.RawMeasurement, error) {

	metricID, ok := metricIDMap[series.Name]
	if !ok {
		return nil, nil
	}

	measurements := make([]entities.RawMeasurement, 0, len(series.DataPoints))

	for _, point := range series.DataPoints {

		timestamp, err := m.timestampParser.Parse(point.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("parse timestamp %q for metric %q: %w", point.Timestamp, series.Name, err)
		}

		measurement := entities.RawMeasurement{
			MeasuringPointID: device.MeasuringPointID,
			MetricID:         metricID,
			Timestamp:        timestamp,
			Value:            point.Value,
			Source:           "dataos",
		}

		measurements = append(measurements, measurement)
	}

	return measurements, nil
}
