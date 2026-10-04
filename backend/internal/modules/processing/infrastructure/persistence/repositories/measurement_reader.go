package repositories

import (
	"context"
	"database/sql"
	"time"

	"gorm.io/gorm"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/infrastructure/persistence/mappers"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/infrastructure/persistence/models"
	"github.com/juanserealpe/agatha-platform/backend/internal/shared/features"
)

// MeasurementReader lee raw_measurements y devices (tablas de Ingestion), solo lectura.
type MeasurementReader struct {
	db  *gorm.DB
	ids mappers.MetricIDs
}

func NewMeasurementReader(db *gorm.DB, ids mappers.MetricIDs) *MeasurementReader {
	return &MeasurementReader{db: db, ids: ids}
}

// GetSamples hace una sola consulta por punto (aprovecha la PK
// (measuring_point_id, metric_id, timestamp)) y pivota en el mapper.
func (r *MeasurementReader) GetSamples(ctx context.Context, pointID int64, from, to time.Time,
) ([]features.Sample, error) {
	var rows []models.RawSampleRowModel

	err := r.db.WithContext(ctx).
		Table("raw_measurements").
		Select("timestamp, metric_id, value").
		Where("measuring_point_id = ?", pointID).
		Where("metric_id IN ?", []uint64{r.ids.B1, r.ids.B2, r.ids.Planitud}).
		Where("timestamp >= ? AND timestamp < ?", from.UTC(), to.UTC()).
		Order("timestamp ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	return mappers.ToSamples(rows, r.ids), nil
}

// GetLatestTimestamp devuelve MAX(timestamp) de B2 del punto (nil si no hay).
func (r *MeasurementReader) GetLatestTimestamp(ctx context.Context, pointID int64) (*time.Time, error) {
	var latest sql.NullTime

	err := r.db.WithContext(ctx).
		Table("raw_measurements").
		Select("MAX(timestamp)").
		Where("measuring_point_id = ? AND metric_id = ?", pointID, r.ids.B2).
		Row().
		Scan(&latest)
	if err != nil {
		return nil, err
	}

	if !latest.Valid {
		return nil, nil
	}

	utc := latest.Time.UTC()
	return &utc, nil
}

// ListActiveMeasuringPoints devuelve los puntos activos de la tabla devices.
func (r *MeasurementReader) ListActiveMeasuringPoints(ctx context.Context) ([]int64, error) {
	var ids []int64

	err := r.db.WithContext(ctx).
		Table("devices").
		Where("active = ?", true).
		Order("measuring_point_id ASC").
		Pluck("measuring_point_id", &ids).Error
	if err != nil {
		return nil, err
	}

	return ids, nil
}
