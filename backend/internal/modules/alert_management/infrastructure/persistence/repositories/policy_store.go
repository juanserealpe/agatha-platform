package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/infrastructure/persistence/mappers"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/infrastructure/persistence/models"
)

// PolicyStore implementa ports.PolicyStore sobre alert_policy_versions.
type PolicyStore struct {
	db *gorm.DB
}

func NewPolicyStore(db *gorm.DB) *PolicyStore {
	return &PolicyStore{db: db}
}

func scope(q *gorm.DB, pointID *int64) *gorm.DB {
	if pointID == nil {
		return q.Where("measuring_point_id IS NULL")
	}
	return q.Where("measuring_point_id = ?", *pointID)
}

func (s *PolicyStore) GetEffective(ctx context.Context, pointID int64, eventType entities.EventType, t time.Time) (*entities.Policy, error) {
	p, err := s.GetEffectiveExact(ctx, &pointID, eventType, t)
	if err != nil || p != nil {
		return p, err
	}
	return s.GetEffectiveExact(ctx, nil, eventType, t)
}

func (s *PolicyStore) GetEffectiveExact(ctx context.Context, pointID *int64, eventType entities.EventType, t time.Time) (*entities.Policy, error) {
	var row models.AlertPolicyVersionModel
	q := scope(s.db.WithContext(ctx).Model(&models.AlertPolicyVersionModel{}), pointID).
		Where("event_type = ? AND valid_from <= ? AND (valid_to IS NULL OR valid_to > ?)", string(eventType), t.UTC(), t.UTC()).
		Order("valid_from DESC").Limit(1)
	err := q.Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p := mappers.ToPolicy(row)
	return &p, nil
}

func (s *PolicyStore) History(ctx context.Context, pointID *int64, eventType entities.EventType) ([]entities.Policy, error) {
	var rows []models.AlertPolicyVersionModel
	err := scope(s.db.WithContext(ctx).Model(&models.AlertPolicyVersionModel{}), pointID).
		Where("event_type = ?", string(eventType)).Order("version ASC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]entities.Policy, len(rows))
	for i, r := range rows {
		out[i] = mappers.ToPolicy(r)
	}
	return out, nil
}

// Create cierra la vigente del alcance e inserta la nueva en UNA transacción
// (SELECT … FOR UPDATE NOWAIT de la abierta). Concurrencia ⇒ ports.ErrConflict.
func (s *PolicyStore) Create(ctx context.Context, p entities.Policy) (entities.Policy, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var open []models.AlertPolicyVersionModel
		if err := scope(tx.Model(&models.AlertPolicyVersionModel{}), p.MeasuringPointID).
			Where("event_type = ? AND valid_to IS NULL", string(p.EventType)).
			Clauses(clause.Locking{Strength: "UPDATE", Options: "NOWAIT"}).Find(&open).Error; err != nil {
			return err
		}
		for _, o := range open {
			if !p.ValidFrom.After(o.ValidFrom) {
				return ports.ErrConflict
			}
			if err := tx.Model(&models.AlertPolicyVersionModel{}).Where("id = ?", o.ID).Update("valid_to", p.ValidFrom.UTC()).Error; err != nil {
				return err
			}
		}
		var maxVersion int
		if err := scope(tx.Model(&models.AlertPolicyVersionModel{}), p.MeasuringPointID).
			Where("event_type = ?", string(p.EventType)).Select("COALESCE(MAX(version), 0)").Scan(&maxVersion).Error; err != nil {
			return err
		}
		p.Version = maxVersion + 1
		row := mappers.ToPolicyModel(p)
		row.ID = 0
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		p.ID = row.ID
		return nil
	})
	if isConflict(err) {
		return entities.Policy{}, ports.ErrConflict
	}
	return p, err
}

// isConflict: NOWAIT (55P03), UNIQUE (23505), EXCLUDE (23P01) o serialización (40001).
func isConflict(err error) bool {
	if errors.Is(err, ports.ErrConflict) {
		return true
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "55P03", "23505", "23P01", "40001":
			return true
		}
	}
	return false
}
