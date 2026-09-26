package entities

import (
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/enums"
)

type IngestionGap struct {
	ID               uint64
	MeasuringPointID uint64
	StartedAt        time.Time
	EndedAt          *time.Time
	ExpectedRecords  int
	ReceivedRecords  int
	Status           enums.GapStatus
	CreatedAt        time.Time
	ResolvedAt       *time.Time
}
