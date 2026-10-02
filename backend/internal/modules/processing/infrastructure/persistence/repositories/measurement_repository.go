package repositories

import (
	"context"

	"gorm.io/gorm"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/infrastructure/persistence/mappers"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/infrastructure/persistence/models"
)

type MeasurementRepository struct {
	db *gorm.DB
}

func NewMeasurementRepository(db *gorm.DB) *MeasurementRepository {
	return &MeasurementRepository{
		db: db,
	}
}

func (r *MeasurementRepository) GetLatestMeasurements(
	ctx context.Context,
	measuringPointID int64,
) ([]entities.RawMeasurement, error) {

	// MAX(timestamp) del punto: la última fecha ingerida. Si no hay filas es NULL
	// y la consulta principal devuelve vacío.
	latestTimestamp := r.db.
		Table("raw_measurements").
		Select("MAX(timestamp)").
		Where("measuring_point_id = ?", measuringPointID)

	var rows []models.LatestMeasurementModel

	err := r.db.WithContext(ctx).
		Table("raw_measurements AS rm").
		Select(`
			rm.measuring_point_id,
			rm.timestamp,
			rm.value,
			rm.source,
			rm.metric_id,
			md.key AS metric_key,
			md.name AS metric_name,
			md.unit
		`).
		Joins(`
			INNER JOIN metric_definitions AS md
				ON md.id = rm.metric_id
		`).
		Where(
			"rm.measuring_point_id = ?",
			measuringPointID,
		).
		Where(
			"rm.timestamp = (?)",
			latestTimestamp,
		).
		Order("rm.metric_id ASC").
		Scan(&rows).Error

	if err != nil {
		return nil, err
	}

	result := make([]entities.RawMeasurement, 0, len(rows))

	for _, row := range rows {
		result = append(
			result,
			mappers.ToRawMeasurement(row),
		)
	}

	return result, nil
}
