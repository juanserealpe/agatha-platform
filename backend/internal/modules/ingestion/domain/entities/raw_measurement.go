package entities

import "time"

type RawMeasurement struct {
	ID          string
	DeviceID    string
	MetricID    uint64
	Timestamp   time.Time
	Value       float64
	Source      string
	ExternalKey string
}
