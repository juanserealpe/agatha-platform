package repositories

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/dashboard/infrastructure/persistence/models"
)

// MeasurementSeriesReader lee las variables de entities.SourceMeasurement
// desde raw_measurements (tabla de Ingestion, solo lectura).
type MeasurementSeriesReader struct {
	db *gorm.DB
}

func NewMeasurementSeriesReader(db *gorm.DB) *MeasurementSeriesReader {
	return &MeasurementSeriesReader{db: db}
}

// Read hace una sola consulta para todas las métricas pedidas.
func (r *MeasurementSeriesReader) Read(
	ctx context.Context,
	deviceID int64,
	definitions []entities.SeriesDefinition,
	startDate time.Time,
	endDate time.Time,
) (map[string][]entities.SeriesPoint, error) {
	keyByMetric := make(map[uint64]string, len(definitions))
	metricIDs := make([]uint64, 0, len(definitions))
	for _, definition := range definitions {
		keyByMetric[definition.MetricID] = definition.Key
		metricIDs = append(metricIDs, definition.MetricID)
	}

	var rows []models.MetricSeriesPointModel

	err := r.db.WithContext(ctx).
		Table("raw_measurements").
		Select("metric_id, timestamp, value").
		Where("measuring_point_id = ?", deviceID).
		Where("metric_id IN ?", metricIDs).
		Where("timestamp >= ? AND timestamp < ?", startDate, endDate).
		Order("timestamp ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	result := make(map[string][]entities.SeriesPoint, len(definitions))
	for _, row := range rows {
		key := keyByMetric[row.MetricID]
		result[key] = append(result[key], entities.SeriesPoint{Timestamp: row.Timestamp.UTC(), Value: row.Value})
	}

	return result, nil
}
