package entities

import "time"

type DeviceSummary struct {
	ID               int64
	Name             string
	LastConnection   *time.Time
	LastBatteryLevel *float64
}
