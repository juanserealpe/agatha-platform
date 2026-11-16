package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
	domainRepositories "github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/repositories"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/infrastructure/persistence/mappers"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/infrastructure/persistence/models"
)

// HistoryResetter implementa domainRepositories.HistoryResetter con GORM. Todo
// el reinicio (leer, borrar filas, borrar versiones, insertar la v1) corre en
// UNA transacción de BD.
type HistoryResetter struct {
	db *gorm.DB
}

func NewHistoryResetter(db *gorm.DB) *HistoryResetter {
	return &HistoryResetter{db: db}
}

func (r *HistoryResetter) Transaction(ctx context.Context, fn func(tx domainRepositories.HistoryResetTx) error) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&historyResetTx{tx: tx})
	})
	if isConflict(err) {
		return fmt.Errorf("%w: %w", domainRepositories.ErrConflict, err)
	}
	return err
}

type historyResetTx struct {
	tx *gorm.DB
}

// LockVersions: SELECT ... FOR UPDATE NOWAIT de todas las versiones del punto.
func (t *historyResetTx) LockVersions(ctx context.Context, pointID int64, eventType entities.EventType) ([]entities.ThresholdVersion, error) {
	var rows []models.RuleThresholdVersionModel
	err := t.tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE", Options: "NOWAIT"}).
		Where("measuring_point_id = ? AND event_type = ?", pointID, string(eventType)).
		Order("version ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return mappers.ToThresholdVersions(rows), nil
}

// FirstMeasurementAt usa la misma expresión que SeedInitialThresholds:
// MIN(raw_measurements.timestamp) del punto.
func (t *historyResetTx) FirstMeasurementAt(ctx context.Context, pointID int64) (*time.Time, error) {
	var first sql.NullTime
	err := t.tx.WithContext(ctx).
		Raw(`SELECT MIN(r.timestamp) FROM raw_measurements r WHERE r.measuring_point_id = ?`, pointID).
		Scan(&first).Error
	if err != nil || !first.Valid {
		return nil, err
	}
	at := first.Time.UTC()
	return &at, nil
}

func (t *historyResetTx) DeleteEvaluations(ctx context.Context, pointID int64, eventType entities.EventType) (int64, error) {
	result := t.tx.WithContext(ctx).
		Where("measuring_point_id = ? AND event_type = ?", pointID, string(eventType)).
		Delete(&models.RuleEvaluationModel{})
	return result.RowsAffected, result.Error
}

func (t *historyResetTx) DeleteVersions(ctx context.Context, pointID int64, eventType entities.EventType) (int64, error) {
	result := t.tx.WithContext(ctx).
		Where("measuring_point_id = ? AND event_type = ?", pointID, string(eventType)).
		Delete(&models.RuleThresholdVersionModel{})
	return result.RowsAffected, result.Error
}

func (t *historyResetTx) InsertVersion(ctx context.Context, v entities.ThresholdVersion) (int64, error) {
	row := mappers.ToThresholdVersionModel(v)
	row.ID = 0
	if err := t.tx.WithContext(ctx).Create(&row).Error; err != nil {
		return 0, err
	}
	return row.ID, nil
}
