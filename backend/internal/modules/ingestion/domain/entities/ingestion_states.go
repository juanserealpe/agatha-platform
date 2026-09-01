package entities

import "time"

type IngestionState struct {
	ID               uint64
	MeasuringPointID uint64
	Source           string
	InitialDate      time.Time
	LastIngestedAt   *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
	ReceivedRecords  int
}
