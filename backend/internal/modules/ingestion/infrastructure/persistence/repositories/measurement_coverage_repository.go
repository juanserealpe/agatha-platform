package repositories

import (
	"context"
	"database/sql"
	"time"

	"gorm.io/gorm"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/persistence/models"
)

// MeasurementCoverageRepository implementa repositories.MeasurementCoverageRepository
// sobre raw_measurements (usa el índice único que empieza por measuring_point_id).
type MeasurementCoverageRepository struct {
	db *gorm.DB
}

func NewMeasurementCoverageRepository(db *gorm.DB) *MeasurementCoverageRepository {
	return &MeasurementCoverageRepository{db: db}
}

func (r *MeasurementCoverageRepository) LatestTimestamp(ctx context.Context, measuringPointID int64) (*time.Time, error) {
	var latest sql.NullTime
	err := r.db.WithContext(ctx).
		Model(&models.RawMeasurementModel{}).
		Select("MAX(timestamp)").
		Where("measuring_point_id = ?", measuringPointID).
		Scan(&latest).Error
	if err != nil {
		return nil, err
	}
	if !latest.Valid {
		return nil, nil
	}
	return &latest.Time, nil
}

func (r *MeasurementCoverageRepository) ListTimestamps(ctx context.Context, measuringPointID int64, from, to time.Time,
) ([]time.Time, error) {
	var timestamps []time.Time
	err := r.db.WithContext(ctx).
		Model(&models.RawMeasurementModel{}).
		Distinct("timestamp").
		Where("measuring_point_id = ? AND timestamp >= ? AND timestamp <= ?", measuringPointID, from, to).
		Order("timestamp").
		Pluck("timestamp", &timestamps).Error
	return timestamps, err
}
