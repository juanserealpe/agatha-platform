package repositories

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/infrastructure/persistence/models"
)

// IndicatorReader implementa ports.IndicatorSource leyendo feature_evaluations
// con un modelo propio (solo lectura; usa el índice único de Processing
// (punto, ventana, hora)).
type IndicatorReader struct {
	db *gorm.DB
}

func NewIndicatorReader(db *gorm.DB) *IndicatorReader {
	return &IndicatorReader{db: db}
}

const (
	sourceWindowHours = 24
	sourceStatus      = "CALCULATED"
)

func (r *IndicatorReader) calculated(ctx context.Context, from, to time.Time) *gorm.DB {
	q := r.db.WithContext(ctx).
		Model(&models.FeatureEvaluationReadModel{}).
		Where("window_hours = ? AND status = ? AND i IS NOT NULL", sourceWindowHours, sourceStatus)
	if !from.IsZero() {
		q = q.Where("evaluated_at >= ?", from.UTC())
	}
	if !to.IsZero() {
		q = q.Where("evaluated_at < ?", to.UTC())
	}
	return q
}

func (r *IndicatorReader) ListPoints(ctx context.Context, from, to time.Time) ([]int64, error) {
	var points []int64
	err := r.calculated(ctx, from, to).
		Distinct().
		Order("measuring_point_id").
		Pluck("measuring_point_id", &points).Error
	return points, err
}

func (r *IndicatorReader) ListCalculated(ctx context.Context, pointID int64, from, to, after time.Time, limit int,
) ([]ports.CalculatedIndicator, error) {
	q := r.calculated(ctx, from, to).Where("measuring_point_id = ?", pointID)
	if !after.IsZero() {
		q = q.Where("evaluated_at > ?", after.UTC())
	}

	var rows []models.FeatureEvaluationReadModel
	if err := q.Order("evaluated_at ASC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}

	out := make([]ports.CalculatedIndicator, 0, len(rows))
	for _, m := range rows {
		if m.I == nil {
			continue
		}
		out = append(out, ports.CalculatedIndicator{
			MeasuringPointID: m.MeasuringPointID,
			EvaluatedAt:      m.EvaluatedAt.UTC(),
			I:                *m.I,
			LowConfidence:    m.LowConfidence,
		})
	}
	return out, nil
}
