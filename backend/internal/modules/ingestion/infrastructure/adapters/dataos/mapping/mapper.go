package mapping

import (
	"fmt"

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
var metricIDMap = map[string]uint64{
	"pitch":               1,
	"roll":                2,
	"voltaje":             3,
	"corriente":           4,
	"bateria":             5,
	"rms":                 6,
	"centroide_espectral": 7,
	"planitud_espectral":  8,
	"deriva":              9,
	"actividad":           10,
	"mag_promedio":        11,
	"act_pico":            12,
	"razib_r":             13,
	"b0_viento_30_300hz":  14,
	"b1_fuga_500_1200hz":  15,
	"b2_fuga_1200_2000hz": 16,
	"b3_guarda_2_2_4khz":  17,
	"pitch_base":          18,
	"roll_base":           19,
	"rms_acustico":        20,
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
) ([]entities.RawMeasurement, error) {

	metricID, ok := metricIDMap[metricKey]
	if !ok {
		return nil, fmt.Errorf(
			"metric ID not found for metric key %q",
			metricKey,
		)
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
			MeasuringPointID: device.MeasuringPointID,
			MetricID:         metricID,
			Timestamp:        timestamp,
			Value:            point.Value,
			Source:           "dataos",
		}

		measurements = append(
			measurements,
			measurement,
		)
	}

	return measurements, nil
}
