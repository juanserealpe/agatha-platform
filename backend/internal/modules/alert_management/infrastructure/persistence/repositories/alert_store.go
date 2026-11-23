// Package repositories implementa los ports de Alert Management con GORM
// sobre la BD compartida (PostgreSQL). Lee rule_evaluations y devices con
// modelos propios de solo lectura.
package repositories

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/infrastructure/persistence"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/infrastructure/persistence/mappers"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/infrastructure/persistence/models"
)

// AlertStore implementa ports.AlertStore y ports.AlertReset.
type AlertStore struct {
	db *gorm.DB
}

func NewAlertStore(db *gorm.DB) *AlertStore {
	return &AlertStore{db: db}
}

func (s *AlertStore) Transaction(ctx context.Context, fn func(tx ports.AlertTx) error) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&alertTx{db: tx})
	})
}

// CountPoint cuenta lo que ResetPoint borraría (§11.2).
func (s *AlertStore) CountPoint(ctx context.Context, pointID int64) (ports.ResetCounts, error) {
	var c ports.ResetCounts
	err := s.db.WithContext(ctx).Raw(`
SELECT
	(SELECT count(*) FROM alerts WHERE measuring_point_id = @p) AS alerts,
	(SELECT count(*) FROM alerts WHERE measuring_point_id = @p AND ended_at IS NULL) AS active,
	(SELECT count(*) FROM alert_messages m JOIN alerts a ON a.id = m.alert_id WHERE a.measuring_point_id = @p) AS messages,
	(SELECT count(*) FROM alerts WHERE measuring_point_id = @p AND classification IS NOT NULL) AS classified,
	(SELECT count(*) FROM alerts WHERE measuring_point_id = @p AND label IS NOT NULL) AS labeled,
	(SELECT count(*) FROM alert_observations o JOIN alerts a ON a.id = o.alert_id WHERE a.measuring_point_id = @p) AS observations`,
		map[string]any{"p": pointID}).Scan(&c).Error
	return c, err
}

// ResetPoint borra TODAS las alertas del punto (con sus colores, mensajes y
// observaciones en cascada) y sus estados de procesamiento de los dos flujos.
// El trigger solo lo permite con SET LOCAL.
func (s *AlertStore) ResetPoint(ctx context.Context, pointID int64) (int64, error) {
	var deleted int64
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SET LOCAL " + persistence.RebuildSetting + " = 'on'").Error; err != nil {
			return err
		}
		res := tx.Where("measuring_point_id = ?", pointID).Delete(&models.AlertModel{})
		if res.Error != nil {
			return res.Error
		}
		deleted = res.RowsAffected
		return tx.Where("measuring_point_id = ?", pointID).Delete(&models.AlertProcessingStateModel{}).Error
	})
	return deleted, err
}

type alertTx struct {
	db *gorm.DB
}

func (t *alertTx) GetState(ctx context.Context, pointID int64, eventType entities.EventType) (*entities.ProcessingState, error) {
	var row models.AlertProcessingStateModel
	err := t.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("measuring_point_id = ? AND event_type = ?", pointID, string(eventType)).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	st := mappers.ToState(row)
	return &st, nil
}

func (t *alertTx) SaveState(ctx context.Context, s entities.ProcessingState) error {
	row := mappers.ToStateModel(s)
	row.UpdatedAt = time.Now().UTC()
	return t.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "measuring_point_id"}, {Name: "event_type"}},
		DoUpdates: clause.AssignmentColumns([]string{"last_evaluated_at", "pending_run_started_at", "last_alert_id", "last_status", "updated_at"}),
	}).Create(&row).Error
}

// GetActive usa el índice único parcial ux_alerts_one_active (punto, evento) WHERE state = 'ACTIVA'.
func (t *alertTx) GetActive(ctx context.Context, pointID int64, eventType entities.EventType) (*entities.Alert, error) {
	var row models.AlertModel
	err := t.db.WithContext(ctx).
		Where("measuring_point_id = ? AND event_type = ? AND state = ?", pointID, string(eventType), string(entities.AlertActive)).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	a := mappers.ToAlert(row)
	return &a, nil
}

func (t *alertTx) GetLastFinished(ctx context.Context, pointID int64, eventType entities.EventType) (*entities.Alert, error) {
	var rows []models.AlertModel
	err := t.db.WithContext(ctx).
		Where("measuring_point_id = ? AND event_type = ? AND state = ? AND ended_at IS NOT NULL",
			pointID, string(eventType), string(entities.AlertFinished)).
		Order("ended_at DESC, id DESC").Limit(1).Find(&rows).Error
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	a := mappers.ToAlert(rows[0])
	return &a, nil
}

func (t *alertTx) GetAlert(ctx context.Context, id int64, forUpdate bool) (*entities.Alert, error) {
	q := t.db.WithContext(ctx)
	if forUpdate {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var row models.AlertModel
	err := q.Where("id = ?", id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ports.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	a := mappers.ToAlert(row)
	return &a, nil
}

func (t *alertTx) InsertAlert(ctx context.Context, a entities.Alert) (int64, error) {
	row := mappers.ToAlertModel(a)
	row.ID = 0
	if err := t.db.WithContext(ctx).Omit(clause.Associations).Create(&row).Error; err != nil {
		return 0, err
	}
	return row.ID, nil
}

func (t *alertTx) UpdateProcessing(ctx context.Context, a entities.Alert) error {
	res := t.db.WithContext(ctx).Model(&models.AlertModel{}).Where("id = ?", a.ID).Updates(mappers.ProcessingColumns(a))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ports.ErrNotFound
	}
	return nil
}

func (t *alertTx) GetCurrentSegment(ctx context.Context, alertID int64) (*entities.Segment, error) {
	var row models.AlertLevelSegmentModel
	err := t.db.WithContext(ctx).Where("alert_id = ?", alertID).Order("seq DESC").Limit(1).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s := mappers.ToSegment(row)
	return &s, nil
}

func (t *alertTx) InsertSegment(ctx context.Context, s entities.Segment) (int64, error) {
	var maxSeq int
	if err := t.db.WithContext(ctx).Model(&models.AlertLevelSegmentModel{}).
		Where("alert_id = ?", s.AlertID).Select("COALESCE(MAX(seq), 0)").Scan(&maxSeq).Error; err != nil {
		return 0, err
	}
	row := mappers.ToSegmentModel(s)
	row.ID, row.Seq = 0, maxSeq+1
	if err := t.db.WithContext(ctx).Omit(clause.Associations).Create(&row).Error; err != nil {
		return 0, err
	}
	return row.ID, nil
}

func (t *alertTx) CloseSegment(ctx context.Context, segmentID int64, at time.Time) error {
	return t.db.WithContext(ctx).Model(&models.AlertLevelSegmentModel{}).
		Where("id = ? AND ended_at IS NULL", segmentID).Update("ended_at", at.UTC()).Error
}

func (t *alertTx) InsertMessage(ctx context.Context, m entities.Message) (int64, bool, error) {
	row := mappers.ToMessageModel(m)
	row.ID = 0
	res := t.db.WithContext(ctx).Omit(clause.Associations).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "alert_id"}, {Name: "evaluated_at"}, {Name: "code"}},
		DoNothing: true,
	}).Create(&row)
	if res.Error != nil {
		return 0, false, res.Error
	}
	if res.RowsAffected > 0 {
		return row.ID, true, nil
	}
	var existing models.AlertMessageModel
	err := t.db.WithContext(ctx).Select("id").
		Where("alert_id = ? AND evaluated_at = ? AND code = ?", m.AlertID, m.EvaluatedAt.UTC(), m.Code).Take(&existing).Error
	return existing.ID, false, err
}

func (t *alertTx) Classify(ctx context.Context, alertID int64, m entities.Management) (bool, error) {
	res := t.db.WithContext(ctx).Model(&models.AlertModel{}).
		Where("id = ? AND classification IS NULL", alertID).
		Updates(map[string]any{
			"management_status":           string(entities.StatusClassifiedNoLabel),
			"classification":              string(m.Classification),
			"classified_at":               m.ClassifiedAt,
			"classified_origin":           string(m.ClassifiedOrigin),
			"classified_device":           nullable(m.ClassifiedDevice),
			"classified_recorded_at":      m.ClassifiedRecordedAt,
			"classified_client_record_id": nullable(m.ClassifiedClientRecordID),
		})
	return res.RowsAffected > 0, res.Error
}

func (t *alertTx) Label(ctx context.Context, alertID int64, m entities.Management) (bool, error) {
	res := t.db.WithContext(ctx).Model(&models.AlertModel{}).
		Where("id = ? AND classification IS NOT NULL AND label IS NULL", alertID).
		Updates(map[string]any{
			"management_status":        string(entities.StatusClassifiedAndLabel),
			"label":                    string(m.Label),
			"label_other_text":         nullable(m.LabelOtherText),
			"labeled_at":               m.LabeledAt,
			"labeled_origin":           string(m.LabeledOrigin),
			"labeled_device":           nullable(m.LabeledDevice),
			"labeled_recorded_at":      m.LabeledRecordedAt,
			"labeled_client_record_id": nullable(m.LabeledClientRecordID),
		})
	return res.RowsAffected > 0, res.Error
}

func (t *alertTx) InsertObservation(ctx context.Context, o entities.Observation) (int64, bool, error) {
	row := mappers.ToObservationModel(o)
	row.ID = 0
	q := t.db.WithContext(ctx).Omit(clause.Associations)
	if row.ClientRecordID != nil {
		q = q.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "client_record_id"}}, DoNothing: true})
	}
	res := q.Create(&row)
	if res.Error != nil {
		return 0, false, res.Error
	}
	return row.ID, res.RowsAffected > 0, nil
}

func (t *alertTx) ClientRecordSeen(ctx context.Context, clientRecordID string) (bool, error) {
	var seen bool
	err := t.db.WithContext(ctx).Raw(`
SELECT EXISTS (SELECT 1 FROM alerts WHERE classified_client_record_id = CAST(@id AS uuid) OR labeled_client_record_id = CAST(@id AS uuid))
    OR EXISTS (SELECT 1 FROM alert_observations WHERE client_record_id = CAST(@id AS uuid))`,
		map[string]any{"id": clientRecordID}).Scan(&seen).Error
	return seen, err
}

// nullable escribe NULL en vez de "" (columnas uuid y opcionales).
func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
