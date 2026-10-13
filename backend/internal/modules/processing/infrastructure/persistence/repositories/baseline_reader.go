package repositories

import (
	"context"
	"database/sql"
	"time"

	"gorm.io/gorm"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/infrastructure/persistence/mappers"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/processing/infrastructure/persistence/models"
)

// BaselineReader implementa ports.BaselineProvider leyendo (solo lectura) las
// tablas baseline_versions y baseline_component_stats del módulo Baseline.
type BaselineReader struct {
	db *gorm.DB
}

func NewBaselineReader(db *gorm.DB) *BaselineReader {
	return &BaselineReader{db: db}
}

// GetVersions devuelve las versiones vigentes en algún instante de [from, to],
// con sus estadísticas, en orden ascendente de valid_from.
func (r *BaselineReader) GetVersions(ctx context.Context, pointID int64, from, to time.Time,
) ([]entities.BaselineSnapshot, error) {
	var rows []models.BaselineVersionReadModel

	err := r.db.WithContext(ctx).
		Preload("ComponentStats").
		Where("measuring_point_id = ? AND valid_from <= ?", pointID, to.UTC()).
		Where("(valid_to IS NULL OR valid_to > ?)", from.UTC()).
		Order("valid_from ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	out := make([]entities.BaselineSnapshot, len(rows))
	for i, row := range rows {
		out[i] = mappers.ToBaselineSnapshot(row)
	}

	return out, nil
}

// GetFirstValidFrom devuelve MIN(valid_from) del punto (nil si no tiene versiones).
func (r *BaselineReader) GetFirstValidFrom(ctx context.Context, pointID int64) (*time.Time, error) {
	var first sql.NullTime

	err := r.db.WithContext(ctx).
		Model(&models.BaselineVersionReadModel{}).
		Select("MIN(valid_from)").
		Where("measuring_point_id = ?", pointID).
		Row().
		Scan(&first)
	if err != nil {
		return nil, err
	}

	if !first.Valid {
		return nil, nil
	}

	utc := first.Time.UTC()
	return &utc, nil
}
