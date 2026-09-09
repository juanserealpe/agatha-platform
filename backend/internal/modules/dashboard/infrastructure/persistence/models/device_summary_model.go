package models

import "time"

type DeviceSummaryModel struct {
	MeasuringPointID int64      `gorm:"column:measuring_point_id"`
	Name             string     `gorm:"column:name"`
	LastConnection   *time.Time `gorm:"column:last_connection"`
	LastBatteryLevel *float64   `gorm:"column:last_battery_level"`
}
