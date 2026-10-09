package entities

import "time"

type IngestionState struct {
	ID               uint64
	MeasuringPointID uint64
	Source           string
	LastIngestedAt   *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
	ReceivedRecords  int
	// RecordID identifica el registro externo que originó el lote (callback de
	// Dataflow Labs). Es nil para los lotes de DataOS.
	RecordID *string
}
