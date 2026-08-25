package repositories

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/persistence/mappers"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/persistence/models"
)

type MeasurementRepository struct {
	db *gorm.DB
}

func NewMeasurementRepository(
	db *gorm.DB,
) *MeasurementRepository {
	return &MeasurementRepository{
		db: db,
	}
}

func (r *MeasurementRepository) Save(
	ctx context.Context,
	entity entities.RawMeasurement,
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

func (r *MeasurementRepository) SaveBatch(
	ctx context.Context,
	entitiesList []entities.RawMeasurement,
) error {

	if len(entitiesList) == 0 {
		return nil
	}

	modelsList := make(
		[]models.RawMeasurementModel,
		0,
		len(entitiesList),
	)

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

	if len(modelsList) == 0 {
		return nil
	}
	err := r.db.
		WithContext(ctx).
		Create(&modelsList).
		Error

	if err != nil {
		return err
	}

	return nil
}
func (r *MeasurementRepository) Exists(
	ctx context.Context,
	deviceID string,
	metricID string,
	timestamp time.Time,
) (bool, error) {

	var count int64

	err := r.db.
		WithContext(ctx).
		Model(&models.RawMeasurementModel{}).
		Where(
			"device_id = ? AND metric_id = ? AND timestamp = ?",
			deviceID,
			metricID,
			timestamp,
		).
		Count(&count).
		Error

	if err != nil {
		return false, err
	}

	return count > 0, nil
}
