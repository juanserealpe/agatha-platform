package repositories

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/infrastructure/persistence/mappers"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/infrastructure/persistence/models"
)

// RuleEvaluationRepository persiste rule_evaluations.
type RuleEvaluationRepository struct {
	db *gorm.DB
}

func NewRuleEvaluationRepository(db *gorm.DB) *RuleEvaluationRepository {
	return &RuleEvaluationRepository{db: db}
}

// GetPosition resuelve "ya existe" y "hay posteriores" en una consulta sobre
// el índice único (punto, evento, hora).
func (r *RuleEvaluationRepository) GetPosition(ctx context.Context, pointID int64, eventType entities.EventType, at time.Time,
) (bool, bool, error) {
	var position struct {
		Found    bool
		HasLater bool
	}
	at = at.UTC()
	err := r.db.WithContext(ctx).
		Model(&models.RuleEvaluationModel{}).
		Select("COALESCE(bool_or(evaluated_at = ?), false) AS found, COALESCE(bool_or(evaluated_at > ?), false) AS has_later", at, at).
		Where("measuring_point_id = ? AND event_type = ? AND evaluated_at >= ?", pointID, string(eventType), at).
		Scan(&position).Error
	return position.Found, position.HasLater, err
}

// GetPrevious usa ix_rule_evaluations_point_event_recent (evaluated_at DESC).
func (r *RuleEvaluationRepository) GetPrevious(ctx context.Context, pointID int64, eventType entities.EventType, before time.Time, limit int,
) ([]entities.RuleEvaluation, error) {
	var rows []models.RuleEvaluationModel
	err := r.db.WithContext(ctx).
		Where("measuring_point_id = ? AND event_type = ? AND evaluated_at < ?", pointID, string(eventType), before.UTC()).
		Order("evaluated_at DESC").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	out := make([]entities.RuleEvaluation, len(rows))
	for i, m := range rows {
		out[i] = mappers.ToRuleEvaluation(m)
	}
	return out, nil
}

// Save inserta UNA fila con ON CONFLICT DO NOTHING en una TX. Con una sola
// fila RowsAffected dice sin ambigüedad si se insertó (y GORM completa el ID
// con RETURNING solo en ese caso).
func (r *RuleEvaluationRepository) Save(ctx context.Context, e entities.RuleEvaluation) (int64, bool, error) {
	row := mappers.ToRuleEvaluationModel(e)
	row.ID = 0

	var inserted bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.
			Omit(clause.Associations).
			Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "measuring_point_id"}, {Name: "event_type"}, {Name: "evaluated_at"}},
				DoNothing: true,
			}).
			Create(&row)
		inserted = result.RowsAffected == 1
		return result.Error
	})
	if err != nil || !inserted {
		return 0, false, err
	}
	return row.ID, true, nil
}

func (r *RuleEvaluationRepository) DeleteRange(ctx context.Context, pointID *int64, eventType entities.EventType, from, to time.Time,
) (int64, error) {
	q := r.db.WithContext(ctx).Where("event_type = ?", string(eventType))
	if pointID != nil {
		q = q.Where("measuring_point_id = ?", *pointID)
	}
	if !from.IsZero() {
		q = q.Where("evaluated_at >= ?", from.UTC())
	}
	if !to.IsZero() {
		q = q.Where("evaluated_at < ?", to.UTC())
	}

	result := q.Delete(&models.RuleEvaluationModel{})
	return result.RowsAffected, result.Error
}
