package models

import "time"

type DeviceModel struct {
	MeasuringPointID int64     `gorm:"column:measuring_point_id;id;primaryKey"`
	Name             string    `gorm:"column:name;not null"`
	Active           bool      `gorm:"column:active;not null"`
	CreatedAt        time.Time `gorm:"column:created_at;not null"`
	UpdatedAt        time.Time `gorm:"column:updated_at;not null"`
}

func (DeviceModel) TableName() string {
	return "devices"
}
