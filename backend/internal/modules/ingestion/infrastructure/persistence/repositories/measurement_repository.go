package repositories

import (
	"context"

	"gorm.io/gorm"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/persistence/mappers"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/persistence/models"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

type MeasurementRepository struct {
	db     *gorm.DB
	logger *logger.Logger
}

func NewMeasurementRepository(db *gorm.DB, logger *logger.Logger) *MeasurementRepository {
	return &MeasurementRepository{db: db, logger: logger}
}

func (r *MeasurementRepository) SaveBatch(ctx context.Context, entitiesList []entities.RawMeasurement,
) (entities.SaveBatchResult, error) {
	if len(entitiesList) == 0 {
		return entities.SaveBatchResult{}, nil
	}

	modelsList := make([]models.RawMeasurementModel, 0, len(entitiesList))
	for i := range entitiesList {
		modelsList = append(modelsList, *mappers.ToRawMeasurementModel(&entitiesList[i]))
	}

	result := r.db.WithContext(ctx).Create(&modelsList)
	if result.Error != nil {
		r.logger.Error("error guardando mediciones", "cantidad", len(modelsList), "error", result.Error)
		return entities.SaveBatchResult{}, result.Error
	}

	// RowsAffected refleja las filas realmente insertadas (también con ON CONFLICT DO NOTHING).
	return entities.SaveBatchResult{Inserted: int(result.RowsAffected)}, nil
}
