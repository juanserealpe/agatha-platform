package models

import "time"

type RawMeasurementModel struct {
	MetricID   uint64    `gorm:"column:metric_id"`
	Timestamp  time.Time `gorm:"column:timestamp"`
	MetricName string    `gorm:"column:metric_name"`
	Unit       string    `gorm:"column:unit"`
	Value      float64   `gorm:"column:value"`
}
