package mapping

import (
	"fmt"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/external/dataos/dashboard"
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

	"Temperatura (°C)": 20,
	"Humedad (%)":      21,
}

var componentMetricIDMap = map[string]uint64{
	"chart_temperature": 20,
	"chart_humidity":    21,
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
) ([]entities.RawMeasurement, error) {

	fmt.Printf(
		"Mapping series: name=%s component=%s unit=%s\n",
		series.Name,
		series.ComponentID,
		series.Unit,
	)

	// Primero intentamos identificar la métrica por el componente.
	// Esto es necesario porque temperatura y humedad llegan desde
	// DataOS con el mismo nombre: "MP-11556".
	metricID, ok := componentMetricIDMap[series.ComponentID]

	if !ok {
		// Para las demás métricas seguimos utilizando el nombre
		// de la serie como hasta ahora.
		metricID, ok = metricIDMap[series.Name]
	}

	if !ok {
		fmt.Printf(
			"Serie no mapeada: name=%s component=%s\n",
			series.Name,
			series.ComponentID,
		)

		return nil, nil
	}

	fmt.Printf(
		"Metric ID for name=%s component=%s: %d\n",
		series.Name,
		series.ComponentID,
		metricID,
	)

	measurements := make([]entities.RawMeasurement, 0, len(series.DataPoints))

	for _, point := range series.DataPoints {

		timestamp, err := m.timestampParser.Parse(point.Timestamp)
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

		measurements = append(measurements, measurement)
	}

	return measurements, nil
}
