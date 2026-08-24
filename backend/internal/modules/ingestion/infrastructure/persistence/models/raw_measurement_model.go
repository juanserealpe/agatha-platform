package models

import "time"

type RawMeasurementModel struct {
	ID          string    `gorm:"column:id;primaryKey"`
	DeviceID    string    `gorm:"column:device_id;not null"`
	MetricID    uint64    `gorm:"column:metric_id;not null"`
	Timestamp   time.Time `gorm:"column:timestamp;not null"`
	Value       float64   `gorm:"column:value;not null"`
	Source      string    `gorm:"column:source;not null"`
	ExternalKey string    `gorm:"column:external_key"`
}

func (RawMeasurementModel) TableName() string {
	return "raw_measurements"
}
