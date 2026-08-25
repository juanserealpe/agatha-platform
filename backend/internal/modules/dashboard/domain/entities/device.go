package entities

import "time"

type Device struct {
	ID               string
	MeasuringPointID int64
	Name             string
	Active           bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
