package repositories

import (
	"context"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
)

type MetricDefinitionRepository interface {
	Save(ctx context.Context, metric entities.MetricDefinition) error
	FindByID(ctx context.Context, id string) (*entities.MetricDefinition, error)
	FindByKey(ctx context.Context, deviceID string, key string) (*entities.MetricDefinition, error)
	ListByDevice(ctx context.Context, deviceID string) ([]entities.MetricDefinition, error)
}
