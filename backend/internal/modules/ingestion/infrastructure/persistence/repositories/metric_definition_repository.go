package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/persistence/mappers"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/persistence/models"
)

type MetricDefinitionRepository struct {
	db *gorm.DB
}

func NewMetricDefinitionRepository(db *gorm.DB) *MetricDefinitionRepository {
	return &MetricDefinitionRepository{
		db: db,
	}
}

func (r *MetricDefinitionRepository) Save(
	ctx context.Context,
	entity entities.MetricDefinition,
) error {
	model := mappers.ToMetricDefinitionModel(&entity)

	return r.db.
		WithContext(ctx).
		Create(model).
		Error
}

func (r *MetricDefinitionRepository) FindByID(
	ctx context.Context,
	id string,
) (*entities.MetricDefinition, error) {
	var model models.MetricDefinitionModel

	err := r.db.
		WithContext(ctx).
		First(&model, "id = ?", id).
		Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return mappers.ToMetricDefinitionEntity(&model), nil
}

func (r *MetricDefinitionRepository) ListByDevice(
	ctx context.Context,
	deviceID string,
) ([]entities.MetricDefinition, error) {

	var modelsList []models.MetricDefinitionModel

	err := r.db.
		WithContext(ctx).
		Where("device_id = ?", deviceID).
		Find(&modelsList).
		Error

	if err != nil {
		return nil, err
	}

	result := make(
		[]entities.MetricDefinition,
		0,
		len(modelsList),
	)

	for i := range modelsList {

		entity := mappers.ToMetricDefinitionEntity(
			&modelsList[i],
		)

		if entity != nil {
			result = append(result, *entity)
		}
	}

	return result, nil
}

func (r *MetricDefinitionRepository) FindByKey(
	ctx context.Context,
	key string,
) (*entities.MetricDefinition, error) {

	var model models.MetricDefinitionModel

	err := r.db.
		WithContext(ctx).
		Where("key = ?", key).
		First(&model).
		Error

	if err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return mappers.ToMetricDefinitionEntity(&model), nil
}
