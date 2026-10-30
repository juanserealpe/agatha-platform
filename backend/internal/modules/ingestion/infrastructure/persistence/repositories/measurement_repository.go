package repositories

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/persistence/mappers"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/persistence/models"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/logger"
)

// saveBatchSize acota cada INSERT: un lote histórico de DataOS (5 días × 21
// métricas) supera el límite de 65.535 parámetros de PostgreSQL.
const saveBatchSize = 1000

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

	// El índice único (measuring_point_id, metric_id, timestamp) descarta las
	// mediciones repetidas sin abortar la transacción.
	result := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "measuring_point_id"}, {Name: "metric_id"}, {Name: "timestamp"}},
			DoNothing: true,
		}).
		CreateInBatches(&modelsList, saveBatchSize)
	if result.Error != nil {
		r.logger.Error("error guardando mediciones", "cantidad", len(modelsList), "error", result.Error)
		return entities.SaveBatchResult{}, result.Error
	}

	// RowsAffected cuenta solo las filas realmente insertadas.
	return entities.SaveBatchResult{Inserted: int(result.RowsAffected)}, nil
}
