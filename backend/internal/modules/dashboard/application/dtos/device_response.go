package dtos

import "time"

type DeviceResponse struct {
	ID               int64      `json:"id"`
	Name             string     `json:"name"`
	LastConnection   *time.Time `json:"lastConnection"`
	LastBatteryLevel *float64   `json:"lastBatteryLevel"`
}
