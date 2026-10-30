package models

import "time"

type RawMeasurementModel struct {
	MeasuringPointID int64     `gorm:"column:measuring_point_id;not null"`
	MetricID         uint64    `gorm:"column:metric_id;not null"`
	Timestamp        time.Time `gorm:"column:timestamp;not null"`
	Value            float64   `gorm:"column:value;not null"`
	Source           string    `gorm:"column:source;not null"`
}

func (RawMeasurementModel) TableName() string {
	return "raw_measurements"
}
