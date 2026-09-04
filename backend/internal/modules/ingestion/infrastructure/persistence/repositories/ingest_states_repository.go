package repositories

import (
	"context"
	"errors"
	"fmt"

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

func (r *IngestionStateRepository) FindLatestByMeasuringPointID(ctx context.Context, measuringPointID int64,
) (*entities.IngestionState, error) {

	fmt.Println("3. Buscando el último estado de ingesta para measuring_point_id= ", measuringPointID)

	var model models.IngestionStateModel

	err := r.db.
		WithContext(ctx).
		Where("measuring_point_id = ?", measuringPointID).
		Order("created_at DESC").
		First(&model).
		Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			fmt.Println(
				"3.1 No se encontró estado previo para measuring_point_id= ",
				measuringPointID,
			)
			return nil, nil
		}

		fmt.Println("3.2 Error al obtener estado:", err)

		return nil, err
	}

	fmt.Printf(
		"3.3 Estado encontrado: id=%v, measuring_point_id=%d, initial_date=%v, last_ingested_at=%v, created_at=%v, updated_at=%v\n",
		model.ID,
		model.MeasuringPointID,
		model.InitialDate,
		model.LastIngestedAt,
		model.CreatedAt,
		model.UpdatedAt,
	)

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

func (r *IngestionStateRepository) Update(ctx context.Context, state *entities.IngestionState,
) error {
	model := mappers.ToIngestionStateModel(state)

	return r.db.
		WithContext(ctx).
		Model(&models.IngestionStateModel{}).
		Where("id = ?", model.ID).
		Updates(model).
		Error
}
