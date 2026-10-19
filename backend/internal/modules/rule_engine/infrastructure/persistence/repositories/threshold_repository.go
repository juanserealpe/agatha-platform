// Package repositories implementa la persistencia del Rule Engine con GORM
// sobre PostgreSQL.
package repositories

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
	domainRepositories "github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/repositories"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/infrastructure/persistence/mappers"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/infrastructure/persistence/models"
)

// ThresholdRepository persiste rule_threshold_versions.
type ThresholdRepository struct {
	db *gorm.DB
}

func NewThresholdRepository(db *gorm.DB) *ThresholdRepository {
	return &ThresholdRepository{db: db}
}

// GetEffectiveAt usa ix_rule_threshold_versions_effective (punto, evento, valid_from DESC).
func (r *ThresholdRepository) GetEffectiveAt(ctx context.Context, pointID int64, eventType entities.EventType, t time.Time,
) (*entities.ThresholdVersion, error) {
	var rows []models.RuleThresholdVersionModel
	err := r.db.WithContext(ctx).
		Where("measuring_point_id = ? AND event_type = ? AND valid_from <= ? AND (valid_to IS NULL OR valid_to > ?)",
			pointID, string(eventType), t.UTC(), t.UTC()).
		Order("valid_from DESC").
		Limit(1).
		Find(&rows).Error
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	v := mappers.ToThresholdVersion(rows[0])
	return &v, nil
}

func (r *ThresholdRepository) GetHistory(ctx context.Context, pointID int64, eventType entities.EventType) ([]entities.ThresholdVersion, error) {
	var rows []models.RuleThresholdVersionModel
	err := r.db.WithContext(ctx).
		Where("measuring_point_id = ? AND event_type = ?", pointID, string(eventType)).
		Order("version ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return mappers.ToThresholdVersions(rows), nil
}

func (r *ThresholdRepository) GetOverlapping(ctx context.Context, pointID int64, eventType entities.EventType, from, to time.Time,
) ([]entities.ThresholdVersion, error) {
	var rows []models.RuleThresholdVersionModel
	err := r.db.WithContext(ctx).
		Where("measuring_point_id = ? AND event_type = ? AND valid_from < ? AND (valid_to IS NULL OR valid_to > ?)",
			pointID, string(eventType), to.UTC(), from.UTC()).
		Order("valid_from ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return mappers.ToThresholdVersions(rows), nil
}

func (r *ThresholdRepository) ListPoints(ctx context.Context, eventType entities.EventType) ([]int64, error) {
	var points []int64
	err := r.db.WithContext(ctx).
		Model(&models.RuleThresholdVersionModel{}).
		Where("event_type = ?", string(eventType)).
		Distinct().
		Order("measuring_point_id").
		Pluck("measuring_point_id", &points).Error
	return points, err
}

// Transaction abre una TX de BD: cerrar la vigente e insertar la nueva se
// confirman juntas o no se confirma ninguna.
func (r *ThresholdRepository) Transaction(ctx context.Context, fn func(tx domainRepositories.ThresholdWriter) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&thresholdWriter{tx: tx})
	})
}

type thresholdWriter struct {
	tx *gorm.DB
}

// LockOpen: SELECT ... FOR UPDATE de la versión abierta.
func (w *thresholdWriter) LockOpen(ctx context.Context, pointID int64, eventType entities.EventType) (*entities.ThresholdVersion, error) {
	var rows []models.RuleThresholdVersionModel
	err := w.tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("measuring_point_id = ? AND event_type = ? AND valid_to IS NULL", pointID, string(eventType)).
		Order("valid_from DESC").
		Limit(1).
		Find(&rows).Error
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	v := mappers.ToThresholdVersion(rows[0])
	return &v, nil
}

// Close es el único UPDATE permitido: solo sobre la versión abierta y solo valid_to.
func (w *thresholdWriter) Close(ctx context.Context, versionID int64, validTo time.Time) error {
	result := w.tx.WithContext(ctx).
		Model(&models.RuleThresholdVersionModel{}).
		Where("id = ? AND valid_to IS NULL", versionID).
		Update("valid_to", validTo.UTC())
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("la versión de umbrales %d no estaba abierta", versionID)
	}
	return nil
}

func (w *thresholdWriter) Insert(ctx context.Context, v entities.ThresholdVersion) (int64, error) {
	row := mappers.ToThresholdVersionModel(v)
	row.ID = 0
	if err := w.tx.WithContext(ctx).Create(&row).Error; err != nil {
		return 0, err
	}
	return row.ID, nil
}
