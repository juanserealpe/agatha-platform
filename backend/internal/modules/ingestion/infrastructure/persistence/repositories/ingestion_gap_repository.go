package repositories

import (
	"context"

	"gorm.io/gorm"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/enums"
	domainRepositories "github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/repositories"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/persistence/mappers"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/persistence/models"
)

type GormIngestionGapRepository struct {
	db *gorm.DB
}

func NewGormIngestionGapRepository(
	db *gorm.DB,
) domainRepositories.IngestionGapRepository {
	return &GormIngestionGapRepository{
		db: db,
	}
}

func (r *GormIngestionGapRepository) FindOpenByMeasuringPointID(
	ctx context.Context,
	measuringPointID int64,
) (*entities.IngestionGap, error) {

	var model models.IngestionGapModel

	err := r.db.WithContext(ctx).
		Where(
			"measuring_point_id = ? AND status = ?",
			measuringPointID,
			string(enums.GapStatusOpen),
		).
		First(&model).
		Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}

		return nil, err
	}

	return mappers.IngestionGapModelToEntity(&model), nil
}

func (r *GormIngestionGapRepository) Save(
	ctx context.Context,
	entity *entities.IngestionGap,
) error {

	model := mappers.IngestionGapToModel(entity)

	return r.db.WithContext(ctx).
		Create(model).
		Error
}

func (r *GormIngestionGapRepository) Update(
	ctx context.Context,
	entity *entities.IngestionGap,
) error {

	model := mappers.IngestionGapToModel(entity)

	return r.db.WithContext(ctx).
		Save(model).
		Error
}
