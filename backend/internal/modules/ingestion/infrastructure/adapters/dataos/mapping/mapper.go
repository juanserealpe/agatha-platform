package mapping

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/adapters/dataos/dashboard"
)

var metricKeyMap = map[string]string{
	"Pitch":                 "pitch",
	"Roll":                  "roll",
	"Voltaje (mV)":          "voltaje",
	"Corriente (mA)":        "corriente",
	"Batería (%)":           "bateria",
	"RMS (ADC)":             "rms",
	"Actividad":             "actividad",
	"Mag promedio (g)":      "mag_promedio",
	"Act. pico (g/s)":       "act_pico",
	"Razon R (fuga/viento)": "razib_r",
	"B0 viento 30-300Hz":    "b0_viento_30_300hz",
	"B1 fuga 500-1200Hz":    "b1_fuga_500_1200hz",
	"B2 fuga 1200-2000Hz":   "b2_fuga_1200_2000hz",
	"B3 guarda 2-2.4kHz":    "b3_guarda_2_2_4khz",
	"Pitch base":            "pitch_base",
	"Roll base":             "roll_base",
	"RMS acustico":          "rms_acustico",
	"Planitud espectral":    "planitud_espectral",
	"Deriva (grados)":       "deriva",
}

func MetricKeyFromDataOS(name string) (string, bool) {
	key, ok := metricKeyMap[name]
	return key, ok
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
			ID:        uuid.NewString(),
			DeviceID:  device.ID,
			MetricID:  metricID,
			Timestamp: timestamp,
			Value:     point.Value,
			Source:    "dataos",
		}

		measurements = append(
			measurements,
			measurement,
		)
	}

	return measurements, nil
}
