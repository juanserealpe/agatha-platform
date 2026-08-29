package entities

import "time"

type RawMeasurement struct {
	Timestamp        time.Time
	MeasuringPointID int64
	MetricID         uint64
	Value            float64
	Source           string
}
