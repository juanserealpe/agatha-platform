package entities

import "time"

// VoltageMetricID es el id de "Voltaje (V)" en el catálogo de métricas.
const VoltageMetricID uint64 = 3

type RawMeasurement struct {
	Timestamp        time.Time
	MeasuringPointID int64
	MetricID         uint64
	Value            float64
	Source           string
	// IngestionBatchID es el id de la fila de ingestion_state_models (batch_seq)
	// del lote que trajo la medición. Nil en los datos anteriores al 2026-10-29.
	IngestionBatchID *int64
}

// NormalizeMeasurementValue ajusta el valor que llega de la fuente antes de
// persistirlo. Única regla hoy: el voltaje llega al doble y se divide entre 2.
// La usan todos los caminos de ingesta (callback y DataOS).
func NormalizeMeasurementValue(metricID uint64, value float64) float64 {
	if metricID == VoltageMetricID {
		return value / 2
	}
	return value
}
