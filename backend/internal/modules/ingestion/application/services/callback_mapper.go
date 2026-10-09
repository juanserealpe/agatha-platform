package services

import (
	"strconv"

	ingestionErrors "github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
)

// callbackMetricIDMap traduce el name de cada variable de Dataflow Labs al id
// del catálogo de métricas (ARQUITECTURA_LITE.md §2.1), el mismo que usaba el
// mapper de DataOS. Las variables fuera del catálogo se omiten y se informan.
var callbackMetricIDMap = map[string]uint64{
	"accel_pitch":       1, // Pitch
	"accel_roll":        2, // Roll
	"power_vbat_mv":     3, // Voltaje (mV)
	"power_ibat_ma":     4, // Corriente (mA)
	"power_battery_pct": 5, // Batería (%)
	"piezo_rms":         6, // RMS (ADC)

	"piezo_activity":      7,  // Actividad (piezoeléctrica, %)
	"accel_mag_avg":       8,  // Mag promedio (g)
	"accel_activity_peak": 9,  // Act. pico (g/s)
	"pspec_ratio":         10, // Razon R (fuga/viento)

	"pspec_b0_wind":  11, // B0 viento 30-300Hz
	"pspec_b1_leak":  12, // B1 fuga 500-1200Hz
	"pspec_b2_leak":  13, // B2 fuga 1200-2000Hz
	"pspec_b3_guard": 14, // B3 guarda 2-2.4kHz

	"accel_base_pitch": 15, // Pitch base
	"accel_base_roll":  16, // Roll base
	"pspec_rms":        17, // RMS acustico

	"pspec_flatness":        18, // Planitud espectral
	"accel_drift_deg_cloud": 19, // Deriva (grados); no accel_drift_deg

	"env_temperature_c": 20, // Temperatura (°C)
	"env_humidity_pct":  21, // Humedad (%)
}

// CallbackMapper convierte las variables de un CallbackRecord en mediciones
// crudas (raw_measurements), una por variable con métrica en el catálogo.
type CallbackMapper struct{}

func NewCallbackMapper() *CallbackMapper {
	return &CallbackMapper{}
}

// Map devuelve las mediciones mapeadas y los nombres de las variables sin
// mapeo, en el orden en que llegaron. Falla solo si un valor no cabe en float64.
func (m *CallbackMapper) Map(record *entities.CallbackRecord) ([]entities.RawMeasurement, []string, error) {
	measurements := make([]entities.RawMeasurement, 0, len(record.Readings))
	var unmapped []string

	for _, reading := range record.Readings {
		metricID, ok := callbackMetricIDMap[reading.Name]
		if !ok {
			unmapped = append(unmapped, reading.Name)
			continue
		}

		// Value ya es un literal numérico JSON válido; solo puede fallar por rango.
		value, err := strconv.ParseFloat(reading.Value, 64)
		if err != nil {
			return nil, nil, ingestionErrors.NewInvalidCallbackPayloadError("readings de " + reading.Name + " fuera de rango")
		}

		measurements = append(measurements, entities.RawMeasurement{
			Timestamp:        reading.Timestamp,
			MeasuringPointID: record.MeasuringPointID,
			MetricID:         metricID,
			Value:            value,
			Source:           CallbackSource,
		})
	}

	return measurements, unmapped, nil
}
