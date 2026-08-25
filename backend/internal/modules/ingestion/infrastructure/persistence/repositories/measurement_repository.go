package repositories

import (
	"context"
	"time"

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

func NewMeasurementRepository(
	db *gorm.DB,
	logger *logger.Logger,
) *MeasurementRepository {
	return &MeasurementRepository{
		db:     db,
		logger: logger,
	}
}

func (r *MeasurementRepository) Save(
	ctx context.Context,
	entity entities.RawMeasurement,
) error {

	r.logger.Info(
		"guardando medición",
		"device_id", entity.DeviceID,
		"metric_id", entity.MetricID,
		"timestamp", entity.Timestamp,
		"value", entity.Value,
		"source", entity.Source,
	)

	model := mappers.ToRawMeasurementModel(&entity)

	if model == nil {
		r.logger.Error(
			"no se pudo mapear la medición a modelo de persistencia",
			"device_id", entity.DeviceID,
			"metric_id", entity.MetricID,
		)

		return nil
	}

	err := r.db.
		WithContext(ctx).
		Create(model).
		Error

	if err != nil {
		r.logger.Error(
			"falló el guardado de la medición",
			"device_id", entity.DeviceID,
			"metric_id", entity.MetricID,
			"timestamp", entity.Timestamp,
			"error", err,
		)

		return err
	}

	r.logger.Info(
		"medición guardada exitosamente",
		"device_id", entity.DeviceID,
		"metric_id", entity.MetricID,
		"timestamp", entity.Timestamp,
	)

	return nil
}

func (r *MeasurementRepository) SaveBatch(
	ctx context.Context,
	entitiesList []entities.RawMeasurement,
) error {

	r.logger.Info(
		"guardando lote de mediciones",
		"measurements", len(entitiesList),
	)

	if len(entitiesList) == 0 {
		r.logger.Warn(
			"se intentó guardar un lote de mediciones vacío",
		)

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
		r.logger.Warn(
			"ninguna medición pudo ser mapeada para persistencia",
			"measurements_received", len(entitiesList),
		)

		return nil
	}

	r.logger.Info(
		"ejecutando persistencia de lote de mediciones",
		"measurements", len(modelsList),
	)

	err := r.db.
		WithContext(ctx).
		Create(&modelsList).
		Error

	if err != nil {
		r.logger.Error(
			"falló la persistencia del lote de mediciones",
			"measurements", len(modelsList),
			"error", err,
		)

		return err
	}

	r.logger.Info(
		"lote de mediciones guardado exitosamente",
		"measurements", len(modelsList),
	)

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
