package entities

import "time"

type RawMeasurement struct {
	ID          string
	DeviceID    string
	MetricID    string
	Timestamp   time.Time
	Value       float64
	Source      string
	ExternalKey string
}
