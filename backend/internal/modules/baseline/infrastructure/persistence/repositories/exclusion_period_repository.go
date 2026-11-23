package repositories

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/baseline/infrastructure/persistence/models"
)

// ExclusionPeriodRepository implementa ExclusionPeriodRepository (escritura)
// y KnownEventsProvider (lectura para el cálculo) sobre
// baseline_exclusion_periods.
type ExclusionPeriodRepository struct {
	db  *gorm.DB
	now func() time.Time
}

func NewExclusionPeriodRepository(db *gorm.DB) *ExclusionPeriodRepository {
	return &ExclusionPeriodRepository{db: db, now: time.Now}
}

// UpsertAlert: INSERT … ON CONFLICT (source_alert_code) DO UPDATE de punto,
// inicio y fin. La nota y created_at se conservan.
func (r *ExclusionPeriodRepository) UpsertAlert(ctx context.Context, p entities.ExclusionPeriod) error {
	now := r.now().UTC()
	code := p.SourceAlertCode
	row := models.ExclusionPeriodModel{
		MeasuringPointID: p.MeasuringPointID,
		StartedAt:        p.StartedAt.UTC(),
		EndedAt:          utcPtr(p.EndedAt),
		Source:           string(entities.ExclusionSourceAlert),
		SourceAlertCode:  &code,
		Note:             p.Note,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "source_alert_code"}},
			DoUpdates: clause.AssignmentColumns([]string{"measuring_point_id", "started_at", "ended_at", "updated_at"}),
		}).
		Create(&row).Error
}

// GetKnownEvents devuelve los períodos del punto que se solapan con
// [from, to), ordenados por inicio; uno en curso llega hasta to.
func (r *ExclusionPeriodRepository) GetKnownEvents(ctx context.Context, pointID int64, from, to time.Time) ([]entities.KnownEvent, error) {
	var rows []models.ExclusionPeriodModel
	err := r.db.WithContext(ctx).
		Where("measuring_point_id = ? AND started_at < ? AND (ended_at IS NULL OR ended_at > ?)", pointID, to.UTC(), from.UTC()).
		Order("started_at, id").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	out := make([]entities.KnownEvent, 0, len(rows))
	for _, row := range rows {
		period := entities.ExclusionPeriod{StartedAt: row.StartedAt, EndedAt: row.EndedAt}
		out = append(out, period.KnownEvent(to))
	}
	return out, nil
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

// CountOpenAlerts cuenta los períodos ALERT del punto sin ended_at.
func (r *ExclusionPeriodRepository) CountOpenAlerts(ctx context.Context, pointID int64) (int, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&models.ExclusionPeriodModel{}).
		Where("measuring_point_id = ? AND source = ? AND ended_at IS NULL", pointID, string(entities.ExclusionSourceAlert)).
		Count(&n).Error
	return int(n), err
}
