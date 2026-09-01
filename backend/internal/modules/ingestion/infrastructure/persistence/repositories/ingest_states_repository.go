package repositories

import (
	"context"
	"errors"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/persistence/mappers"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/persistence/models"
	"gorm.io/gorm"
)

type IngestionStateRepository struct {
	db *gorm.DB
}

func NewIngestionStateRepository(db *gorm.DB) *IngestionStateRepository {
	return &IngestionStateRepository{
		db: db,
	}
}

func (r *IngestionStateRepository) FindLatestByMeasuringPointID(
	ctx context.Context,
	measuringPointID int64,
) (*entities.IngestionState, error) {
	var model models.IngestionStateModel

	err := r.db.
		WithContext(ctx).
		Where("measuring_point_id = ?", measuringPointID).
		Order("created_at DESC").
		First(&model).
		Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return mappers.ToIngestionStateEntity(&model), nil
}

func (r *IngestionStateRepository) Save(
	ctx context.Context,
	state *entities.IngestionState,
) error {
	model := mappers.ToIngestionStateModel(state)

	return r.db.
		WithContext(ctx).
		Create(model).
		Error
}

func (r *IngestionStateRepository) Update(
	ctx context.Context,
	state *entities.IngestionState,
) error {
	model := mappers.ToIngestionStateModel(state)

	return r.db.
		WithContext(ctx).
		Model(&models.IngestionStateModel{}).
		Where("id = ?", model.ID).
		Updates(model).
		Error
}
