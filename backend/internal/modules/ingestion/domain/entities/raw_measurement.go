package entities

import "time"

type RawMeasurement struct {
	ID         string
	DeviceID   string
	Timestamp  time.Time
	Payload    map[string]interface{}
	ReceivedAt time.Time
}
