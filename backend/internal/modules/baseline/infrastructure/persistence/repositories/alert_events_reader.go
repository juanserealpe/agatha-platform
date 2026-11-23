package repositories

import (
	"context"

	"gorm.io/gorm"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/infrastructure/persistence/models"
)

// AlertIntervalReader implementa AlertIntervalReader leyendo alerts (tabla de
// Alert Management, solo lectura, modelo propio) para el seed de
// baseline_exclusion_periods. Lee TODAS las alertas, cualquiera sea su tipo o
// clasificación. Baseline ya NO calcula con alerts: solo copia sus intervalos.
type AlertIntervalReader struct {
	db *gorm.DB
}

func NewAlertIntervalReader(db *gorm.DB) *AlertIntervalReader {
	return &AlertIntervalReader{db: db}
}

func (r *AlertIntervalReader) ListAlertIntervals(ctx context.Context) ([]entities.ExclusionPeriod, error) {
	var rows []models.AlertSeedRowModel
	err := r.db.WithContext(ctx).Raw(`
SELECT code, measuring_point_id, started_at, ended_at
  FROM alerts
 ORDER BY id`).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	out := make([]entities.ExclusionPeriod, 0, len(rows))
	for _, row := range rows {
		out = append(out, entities.ExclusionPeriod{
			MeasuringPointID: row.MeasuringPointID,
			StartedAt:        row.StartedAt.UTC(),
			EndedAt:          utcPtr(row.EndedAt),
			Source:           entities.ExclusionSourceAlert,
			SourceAlertCode:  row.Code,
		})
	}
	return out, nil
}
