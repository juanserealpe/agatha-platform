package models

import "time"

// SeriesPointModel es un punto (instante, valor) de una variable leída de
// feature_evaluations.
type SeriesPointModel struct {
	Timestamp time.Time `gorm:"column:timestamp"`
	Value     float64   `gorm:"column:value"`
}

// MetricSeriesPointModel es un punto de una métrica de raw_measurements.
type MetricSeriesPointModel struct {
	MetricID  uint64    `gorm:"column:metric_id"`
	Timestamp time.Time `gorm:"column:timestamp"`
	Value     float64   `gorm:"column:value"`
}
