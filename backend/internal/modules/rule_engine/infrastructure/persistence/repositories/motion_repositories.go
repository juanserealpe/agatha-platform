package repositories

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/application/ports"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
	domainRepositories "github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/repositories"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/infrastructure/persistence/mappers"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/infrastructure/persistence/models"
)

// MotionThresholdRepository persiste motion_threshold_versions.
type MotionThresholdRepository struct {
	db *gorm.DB
}

func NewMotionThresholdRepository(db *gorm.DB) *MotionThresholdRepository {
	return &MotionThresholdRepository{db: db}
}

func (r *MotionThresholdRepository) GetEffectiveAt(ctx context.Context, pointID int64, t time.Time) (*entities.MotionThresholdVersion, error) {
	var rows []models.MotionThresholdVersionModel
	err := r.db.WithContext(ctx).Preload("Rules", orderRules).
		Where("measuring_point_id = ? AND valid_from <= ? AND (valid_to IS NULL OR valid_to > ?)", pointID, t.UTC(), t.UTC()).
		Order("valid_from DESC").Limit(1).Find(&rows).Error
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	v := mappers.ToMotionThresholdVersion(rows[0])
	return &v, nil
}

func (r *MotionThresholdRepository) GetHistory(ctx context.Context, pointID int64) ([]entities.MotionThresholdVersion, error) {
	var rows []models.MotionThresholdVersionModel
	if err := r.db.WithContext(ctx).Preload("Rules", orderRules).Where("measuring_point_id = ?", pointID).Order("version ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return mappers.ToMotionThresholdVersions(rows), nil
}

func (r *MotionThresholdRepository) ListPoints(ctx context.Context) ([]int64, error) {
	var points []int64
	err := r.db.WithContext(ctx).Model(&models.MotionThresholdVersionModel{}).
		Distinct().Order("measuring_point_id").Pluck("measuring_point_id", &points).Error
	return points, err
}

// Transaction traduce los errores de concurrencia de PostgreSQL a ErrConflict.
func (r *MotionThresholdRepository) Transaction(ctx context.Context, fn func(tx domainRepositories.MotionThresholdWriter) error) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&motionThresholdWriter{tx: tx})
	})
	if isConflict(err) {
		return fmt.Errorf("%w: %w", domainRepositories.ErrConflict, err)
	}
	return err
}

type motionThresholdWriter struct {
	tx *gorm.DB
}

func (w *motionThresholdWriter) LockOpen(ctx context.Context, pointID int64) (*entities.MotionThresholdVersion, error) {
	var rows []models.MotionThresholdVersionModel
	err := w.tx.WithContext(ctx).Preload("Rules", orderRules).
		Clauses(clause.Locking{Strength: "UPDATE", Options: "NOWAIT"}).
		Where("measuring_point_id = ? AND valid_to IS NULL", pointID).
		Order("valid_from DESC").Limit(1).Find(&rows).Error
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	v := mappers.ToMotionThresholdVersion(rows[0])
	return &v, nil
}

func (w *motionThresholdWriter) Close(ctx context.Context, versionID int64, validTo time.Time) error {
	result := w.tx.WithContext(ctx).Model(&models.MotionThresholdVersionModel{}).
		Where("id = ? AND valid_to IS NULL", versionID).Update("valid_to", validTo.UTC())
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("la versión de movimiento %d no estaba abierta", versionID)
	}
	return nil
}

func (w *motionThresholdWriter) Insert(ctx context.Context, v entities.MotionThresholdVersion) (int64, error) {
	row := mappers.ToMotionThresholdVersionModel(v)
	row.ID = 0
	if err := w.tx.WithContext(ctx).Create(&row).Error; err != nil {
		return 0, err
	}
	return row.ID, nil
}

// orderRules ordena la tabla de lo normal de una versión.
func orderRules(db *gorm.DB) *gorm.DB { return db.Order("window_hours ASC, axis ASC") }

// MotionEvaluationRepository persiste motion_evaluations.
type MotionEvaluationRepository struct {
	db *gorm.DB
}

func NewMotionEvaluationRepository(db *gorm.DB) *MotionEvaluationRepository {
	return &MotionEvaluationRepository{db: db}
}

// GetLastBefore usa ux_motion_evaluations_point_hour (punto, evaluated_at DESC).
func (r *MotionEvaluationRepository) GetLastBefore(ctx context.Context, pointID int64, before time.Time) (*entities.MotionEvaluation, error) {
	q := r.db.WithContext(ctx).Where("measuring_point_id = ?", pointID)
	if !before.IsZero() {
		q = q.Where("evaluated_at < ?", before.UTC())
	}
	var rows []models.MotionEvaluationModel
	if err := q.Order("evaluated_at DESC").Limit(1).Find(&rows).Error; err != nil || len(rows) == 0 {
		return nil, err
	}
	e := mappers.ToMotionEvaluation(rows[0])
	return &e, nil
}

// Save inserta UNA fila con ON CONFLICT (punto, hora) DO NOTHING.
func (r *MotionEvaluationRepository) Save(ctx context.Context, e entities.MotionEvaluation) (int64, bool, error) {
	row := mappers.ToMotionEvaluationModel(e)
	row.ID = 0
	result := r.db.WithContext(ctx).
		Omit(clause.Associations).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "measuring_point_id"}, {Name: "evaluated_at"}},
			DoNothing: true,
		}).
		Create(&row)
	if result.Error != nil || result.RowsAffected != 1 {
		return 0, false, result.Error
	}
	return row.ID, true, nil
}

func (r *MotionEvaluationRepository) DeleteRange(ctx context.Context, pointID int64, from, to time.Time) (int64, error) {
	q := r.db.WithContext(ctx).Where("measuring_point_id = ?", pointID)
	if !from.IsZero() {
		q = q.Where("evaluated_at >= ?", from.UTC())
	}
	if !to.IsZero() {
		q = q.Where("evaluated_at < ?", to.UTC())
	}
	result := q.Delete(&models.MotionEvaluationModel{})
	return result.RowsAffected, result.Error
}

func (r *MotionEvaluationRepository) ListRange(ctx context.Context, pointID int64, from, to time.Time) ([]entities.MotionEvaluation, error) {
	var rows []models.MotionEvaluationModel
	err := r.db.WithContext(ctx).
		Where("measuring_point_id = ? AND evaluated_at >= ? AND evaluated_at <= ?", pointID, from.UTC(), to.UTC()).
		Order("evaluated_at ASC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]entities.MotionEvaluation, len(rows))
	for i, m := range rows {
		out[i] = mappers.ToMotionEvaluation(m)
	}
	return out, nil
}

// ListEpisodes arma los episodios cerrados del punto con las horas CLEARED
// anteriores a before (cero = sin límite), ascendentes.
func (r *MotionEvaluationRepository) ListEpisodes(ctx context.Context, pointID int64, before time.Time) ([]entities.MotionEpisode, error) {
	q := r.db.WithContext(ctx).Where("measuring_point_id = ? AND episode_event = ?", pointID, string(entities.EpisodeEventCleared))
	if !before.IsZero() {
		q = q.Where("evaluated_at < ?", before.UTC())
	}
	var rows []models.MotionEvaluationModel
	if err := q.Order("evaluated_at ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]entities.MotionEpisode, 0, len(rows))
	for _, m := range rows {
		if ep, ok := entities.EpisodeFromCleared(mappers.ToMotionEvaluation(m)); ok {
			out = append(out, ep)
		}
	}
	return out, nil
}

func (r *MotionEvaluationRepository) CountAfter(ctx context.Context, pointID int64, at time.Time) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&models.MotionEvaluationModel{}).
		Where("measuring_point_id = ? AND evaluated_at >= ?", pointID, at.UTC()).Count(&n).Error
	return n, err
}

// RawMeasurementReader implementa ports.RawMeasurementSource con un modelo
// propio de solo lectura sobre raw_measurements (índice único de Ingestion
// (punto, métrica, timestamp)) y devices.
type RawMeasurementReader struct {
	db *gorm.DB
}

func NewRawMeasurementReader(db *gorm.DB) *RawMeasurementReader {
	return &RawMeasurementReader{db: db}
}

var motionMetrics = []int64{ports.MetricPitch, ports.MetricRoll, ports.MetricDeriva, ports.MetricTemperature}

// ListRecords agrupa por timestamp las cuatro métricas, ascendentes.
func (r *RawMeasurementReader) ListRecords(ctx context.Context, pointID int64, from, to time.Time) ([]entities.RawRecord, error) {
	var rows []models.RawMeasurementReadModel
	err := r.db.WithContext(ctx).
		Select("timestamp", "metric_id", "value").
		Where("measuring_point_id = ? AND metric_id IN ? AND timestamp >= ? AND timestamp < ?", pointID, motionMetrics, from.UTC(), to.UTC()).
		Order("timestamp ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return GroupRawRecords(rows), nil
}

// GroupRawRecords junta las filas (ordenadas por timestamp) en un registro por timestamp.
func GroupRawRecords(rows []models.RawMeasurementReadModel) []entities.RawRecord {
	var out []entities.RawRecord
	for _, m := range rows {
		ts := m.Timestamp.UTC()
		if len(out) == 0 || !out[len(out)-1].Timestamp.Equal(ts) {
			out = append(out, entities.RawRecord{Timestamp: ts})
		}
		rec := &out[len(out)-1]
		v := m.Value
		switch m.MetricID {
		case ports.MetricPitch:
			rec.Pitch = &v
		case ports.MetricRoll:
			rec.Roll = &v
		case ports.MetricDeriva:
			rec.Deriva = &v
		case ports.MetricTemperature:
			rec.Temp = &v
		}
	}
	return out
}

func (r *RawMeasurementReader) FirstMeasurementAt(ctx context.Context, pointID int64) (*time.Time, error) {
	var first *time.Time
	err := r.db.WithContext(ctx).Model(&models.RawMeasurementReadModel{}).
		Select("MIN(timestamp)").
		Where("measuring_point_id = ? AND metric_id IN ?", pointID, []int64{ports.MetricPitch, ports.MetricRoll}).
		Scan(&first).Error
	if err != nil || first == nil {
		return nil, err
	}
	t := first.UTC()
	return &t, nil
}

func (r *RawMeasurementReader) ListPoints(ctx context.Context) ([]int64, error) {
	var points []int64
	err := r.db.WithContext(ctx).Table("devices").Distinct().Order("measuring_point_id").Pluck("measuring_point_id", &points).Error
	return points, err
}
