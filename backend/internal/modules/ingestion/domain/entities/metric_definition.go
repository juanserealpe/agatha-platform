package entities

import (
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/enums"
)

type MetricDefinition struct {
	ID        string
	Key       string
	Name      string
	Unit      string
	Category  enums.MetricCategory
	Active    bool
	CreatedAt time.Time
	UpdatedAt time.Time
}
