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

func NewMeasurementRepository(db *gorm.DB,
	logger *logger.Logger,
) *MeasurementRepository {
	return &MeasurementRepository{
		db:     db,
		logger: logger,
	}
}

func (r *MeasurementRepository) Save(ctx context.Context, entity entities.RawMeasurement,
) error {
	model := mappers.ToRawMeasurementModel(&entity)

	if model == nil {
		return nil
	}

	err := r.db.
		WithContext(ctx).
		Create(model).
		Error

	if err != nil {
		return err
	}

	return nil
}

func (r *MeasurementRepository) SaveBatch(ctx context.Context, entitiesList []entities.RawMeasurement,
) (entities.SaveBatchResult, error) {
	r.logger.Info("[SAVE BATCH] Iniciando proceso de guardado de mediciones en la base de datos", "cantidad", len(entitiesList))
	if len(entitiesList) == 0 {
		r.logger.Warn("[SAVE BATCH] No hay mediciones para guardar en la base de datos")
		return entities.SaveBatchResult{}, nil
	}

	modelsList := make(
		[]models.RawMeasurementModel,
		0,
		len(entitiesList),
	)
	r.logger.Info("[SAVE BATCH] iniciando el MAPPER")
	for i := range entitiesList {
		model := mappers.ToRawMeasurementModel(
			&entitiesList[i],
		)
		if model != nil {
			modelsList = append(
				modelsList,
				*model,
			)
		}
	}
	r.logger.Info("[SAVE BATCH] finalizó el MAPPER", "cantidad", len(modelsList))
	if len(modelsList) == 0 {
		r.logger.Warn("[SAVE BATCH] No hay modelos para guardar en la base de datos")
		return entities.SaveBatchResult{}, nil
	}
	result := r.db.
		WithContext(ctx).
		Create(&modelsList)

	if result.Error != nil {
		r.logger.Error("[SAVE BATCH] Error al guardar las mediciones en la base de datos", "error", result.Error)
		return entities.SaveBatchResult{}, result.Error
	}

	// RowsAffected refleja las filas realmente insertadas (también con ON CONFLICT DO NOTHING).
	return entities.SaveBatchResult{Inserted: int(result.RowsAffected)}, nil
}
