package repositories

import (
	"context"
	"time"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/infrastructure/persistence/mappers"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/infrastructure/persistence/models"

	"gorm.io/gorm"
)

type MeasurementRepository struct {
	db *gorm.DB
}

func NewMeasurementRepository(db *gorm.DB) *MeasurementRepository {
	return &MeasurementRepository{
		db: db,
	}
}

func (r *MeasurementRepository) GetMeasurements(
	ctx context.Context,
	deviceID int64,
	startDate time.Time,
	endDate time.Time,
) ([]entities.RawMeasurement, error) {

	var rows []models.RawMeasurementModel

	err := r.db.WithContext(ctx).
		Table("raw_measurements AS rm").
		Select(`
			rm.metric_id,
			rm.timestamp,
			md.name AS metric_name,
			md.unit,
			rm.value
		`).
		Joins(`
			INNER JOIN metric_definitions AS md
				ON md.id = rm.metric_id
		`).
		Where(
			"rm.measuring_point_id = ?",
			deviceID,
		).
		Where(
			"rm.timestamp >= ?",
			startDate,
		).
		Where(
			"rm.timestamp < ?",
			endDate,
		).
		Order("rm.timestamp ASC").
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
