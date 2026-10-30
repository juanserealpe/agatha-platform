package mapping

import (
	"fmt"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/external/dataos/dashboard"
)

// metricIDMap traduce el nombre de cada serie de DataOS al id del catálogo de
// métricas (ARQUITECTURA_LITE.md §2.1).
var metricIDMap = map[string]uint64{
	"Pitch":          1,
	"Roll":           2,
	"Voltaje (V)":    3,
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

// componentMetricIDMap identifica por componente las series que DataOS nombra
// igual ("MP-11556" para temperatura y humedad).
var componentMetricIDMap = map[string]uint64{
	"chart_temperature": 20,
	"chart_humidity":    21,
}

type Mapper struct {
	timestampParser *TimestampParser
}

func NewMapper(timestampParser *TimestampParser) *Mapper {
	return &Mapper{timestampParser: timestampParser}
}

// MapSeries convierte una serie en mediciones crudas. Devuelve nil, nil si la
// serie no está en el catálogo.
func (m *Mapper) MapSeries(device *entities.Device, series dashboard.Series) ([]entities.RawMeasurement, error) {
	// Primero intentamos identificar la métrica por el componente.
	// Esto es necesario porque temperatura y humedad llegan desde
	// DataOS con el mismo nombre: "MP-11556".
	metricID, ok := componentMetricIDMap[series.ComponentID]

	if !ok {
		// Para las demás métricas seguimos utilizando el nombre
		// de la serie como hasta ahora.
		metricID, ok = metricIDMap[series.Name]
	}

	// Serie fuera del catálogo: el cliente la informa y la omite.
	if !ok {
		return nil, nil
	}

	measurements := make([]entities.RawMeasurement, 0, len(series.DataPoints))

	for _, point := range series.DataPoints {
		timestamp, err := m.timestampParser.Parse(point.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("timestamp %q inválido en la serie %q: %w", point.Timestamp, series.Name, err)
		}

		measurements = append(measurements, entities.RawMeasurement{
			MeasuringPointID: device.MeasuringPointID,
			MetricID:         metricID,
			Timestamp:        timestamp,
			Value:            entities.NormalizeMeasurementValue(metricID, point.Value),
			Source:           "dataos",
		})
	}

	return measurements, nil
}
