package entities

import "time"

type RawMeasurement struct {
	MetricID   uint64
	Timestamp  time.Time
	MetricName string
	Unit       string
	Value      float64
}
