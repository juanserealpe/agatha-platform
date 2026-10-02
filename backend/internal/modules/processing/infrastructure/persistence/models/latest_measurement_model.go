package models

import "time"

// LatestMeasurementModel es la fila del JOIN raw_measurements + metric_definitions.
type LatestMeasurementModel struct {
	MeasuringPointID int64     `gorm:"column:measuring_point_id"`
	Timestamp        time.Time `gorm:"column:timestamp"`
	Value            float64   `gorm:"column:value"`
	Source           string    `gorm:"column:source"`
	MetricID         uint64    `gorm:"column:metric_id"`
	MetricKey        string    `gorm:"column:metric_key"`
	MetricName       string    `gorm:"column:metric_name"`
	Unit             string    `gorm:"column:unit"`
}
