package repositories

import (
	"context"
	"errors"
	"fmt"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/persistence/mappers"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/persistence/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

		fmt.Println("3.2 Error al obtener estado:", err)

		return nil, err
	}
	return mappers.ToIngestionStateEntity(&model), nil
}

func (r *IngestionStateRepository) Save(ctx context.Context, state *entities.IngestionState,
) (entities.SaveStateResult, error) {
	model := mappers.ToIngestionStateModel(state)

	if err := r.db.WithContext(ctx).Create(model).Error; err != nil {
		return entities.SaveStateResult{}, err
	}

	// ingestion_logs es append-only (una fila por lote): el id autoincremental
	// asignado dentro de la transacción es el cursor monótono del lote.
	result := entities.SaveStateResult{BatchSeq: int64(model.ID)}
	if model.LastIngestedAt != nil {
		result.LastIngestedAt = *model.LastIngestedAt
	}

	return result, nil
}

// SaveIfAbsent inserta con ON CONFLICT (record_id) DO NOTHING. Ante dos TX
// concurrentes con el mismo record_id, la segunda espera al commit de la primera
// y no inserta nada (RowsAffected = 0); si la primera se revierte, la segunda inserta.
func (r *IngestionStateRepository) SaveIfAbsent(ctx context.Context, state *entities.IngestionState,
) (entities.SaveStateResult, bool, error) {
	if state == nil || state.RecordID == nil || *state.RecordID == "" {
		return entities.SaveStateResult{}, false, errors.New("record_id requerido para SaveIfAbsent")
	}

	model := mappers.ToIngestionStateModel(state)

	result := r.db.
		WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "record_id"}},
			DoNothing: true,
		}).
		Create(model)

	if result.Error != nil {
		return entities.SaveStateResult{}, false, result.Error
	}

	if result.RowsAffected == 0 {
		return entities.SaveStateResult{}, false, nil
	}

	saved := entities.SaveStateResult{BatchSeq: int64(model.ID)}
	if model.LastIngestedAt != nil {
		saved.LastIngestedAt = *model.LastIngestedAt
	}

	return saved, true, nil
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
