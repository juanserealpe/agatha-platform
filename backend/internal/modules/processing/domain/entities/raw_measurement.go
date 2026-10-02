package entities

import "time"

// RawMeasurement es la vista de Processing de una medición cruda
// (tabla raw_measurements) junto con la definición de su variable.
type RawMeasurement struct {
	Timestamp        time.Time
	MeasuringPointID int64
	Value            float64
	Source           string
	Metric           MetricDefinition
}
