package repositories

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/infrastructure/persistence/mappers"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/infrastructure/persistence/models"
)

// LevelSource lee rule_evaluations + rule_threshold_versions (del Rule
// Engine, solo lectura: acople de esquema, como el Rule Engine con
// feature_evaluations). Implementa ports.LevelSource.
type LevelSource struct {
	db *gorm.DB
}

func NewLevelSource(db *gorm.DB) *LevelSource {
	return &LevelSource{db: db}
}

func (s *LevelSource) ListPoints(ctx context.Context) ([]int64, error) {
	var points []int64
	err := s.db.WithContext(ctx).Table("rule_evaluations").
		Where("event_type = ?", string(entities.EventTypeLeak)).
		Distinct("measuring_point_id").Order("measuring_point_id").Pluck("measuring_point_id", &points).Error
	return points, err
}

func (s *LevelSource) ListAfter(ctx context.Context, pointID int64, after, to time.Time, limit int) ([]entities.LevelNotice, error) {
	q := s.db.WithContext(ctx).Table("rule_evaluations r").
		Select(`r.id, r.measuring_point_id, r.event_type, r.evaluated_at, r.indicator_value, r.raw_level,
			r.computed_level, r.level, r.in_episode, r.hours_above, r.run_amarillo, r.run_naranja, r.below_run,
			r.threshold_version_id, r.low_confidence, r.episode_event, r.reasons::text AS reasons,
			r.d_pl, v.d_pl_min, v.amarillo, v.naranja, v.rojo, v.exit_margin, v.escalate_amarillo_after_hours,
			v.escalate_naranja_after_hours, v.persistence_mode, v.persistence_min_hours`).
		Joins("JOIN rule_threshold_versions v ON v.id = r.threshold_version_id").
		Where("r.measuring_point_id = ? AND r.event_type = ?", pointID, string(entities.EventTypeLeak))
	if !after.IsZero() {
		q = q.Where("r.evaluated_at > ?", after.UTC())
	}
	if !to.IsZero() {
		q = q.Where("r.evaluated_at < ?", to.UTC())
	}
	var rows []models.RuleEvaluationReadModel
	if err := q.Order("r.evaluated_at ASC").Limit(limit).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]entities.LevelNotice, len(rows))
	for i, r := range rows {
		out[i] = mappers.ToLevelNotice(r)
	}
	return out, nil
}

// DeviceDirectory lee devices (de Ingestion). Sin la tabla devuelve nil (los
// mensajes usan MP-<id>). Implementa ports.DeviceDirectory.
type DeviceDirectory struct {
	db *gorm.DB
}

func NewDeviceDirectory(db *gorm.DB) *DeviceDirectory {
	return &DeviceDirectory{db: db}
}

func (d *DeviceDirectory) Get(ctx context.Context, pointID int64) (*entities.Device, error) {
	if !d.db.Migrator().HasTable("devices") {
		return nil, nil
	}
	var row models.DeviceReadModel
	err := d.db.WithContext(ctx).Table("devices").Select("measuring_point_id, name").
		Where("measuring_point_id = ?", pointID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &entities.Device{MeasuringPointID: row.MeasuringPointID, PK: row.Name}, nil
}

// Names devuelve el nombre (PK) de los puntos dados.
func (d *DeviceDirectory) Names(ctx context.Context, pointIDs []int64) (map[int64]string, error) {
	out := map[int64]string{}
	if len(pointIDs) == 0 || !d.db.Migrator().HasTable("devices") {
		return out, nil
	}
	var rows []models.DeviceReadModel
	if err := d.db.WithContext(ctx).Table("devices").Select("measuring_point_id, name").
		Where("measuring_point_id IN ?", pointIDs).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.MeasuringPointID] = r.Name
	}
	return out, nil
}

// MotionSource lee motion_evaluations + motion_threshold_versions (del Rule
// Engine, solo lectura). Solo devuelve horas de versiones con alerts_enabled:
// el modo sombra no llega a Alert Management. Implementa ports.MotionSource.
type MotionSource struct {
	db *gorm.DB
}

func NewMotionSource(db *gorm.DB) *MotionSource {
	return &MotionSource{db: db}
}

// available indica si las tablas de movimiento existen (Rule Engine anterior a v7: no hay flujo).
func (s *MotionSource) available() bool {
	m := s.db.Migrator()
	return m.HasTable("motion_evaluations") && m.HasTable("motion_threshold_versions")
}

func (s *MotionSource) ListPoints(ctx context.Context) ([]int64, error) {
	if !s.available() {
		return nil, nil
	}
	var points []int64
	err := s.db.WithContext(ctx).Table("motion_evaluations e").
		Joins("JOIN motion_threshold_versions v ON v.id = e.threshold_version_id").
		Where("v.alerts_enabled").
		Distinct("e.measuring_point_id").Order("e.measuring_point_id").Pluck("e.measuring_point_id", &points).Error
	return points, err
}

func (s *MotionSource) ListAfter(ctx context.Context, pointID int64, after, to time.Time, limit int) ([]entities.MotionNotice, error) {
	if !s.available() {
		return nil, nil
	}
	q := s.db.WithContext(ctx).Table("motion_evaluations e").
		Select(`e.id, e.measuring_point_id, e.evaluated_at, e.status, e.motion_state, e.temp_state, v.temp_mode,
			e.computed_level, e.in_episode, e.episode_event, e.displaced, e.pitch_now, e.roll_now, e.ref_pitch, e.ref_roll,
			e.shift_started_at, e.gap_from, e.gap_to, e.temp_stable_from, e.temp_stable_to, e.reasons::text AS reasons,
			e.threshold_version_id, v.alerts_enabled`).
		Joins("JOIN motion_threshold_versions v ON v.id = e.threshold_version_id").
		Where("e.measuring_point_id = ? AND v.alerts_enabled", pointID)
	if !after.IsZero() {
		q = q.Where("e.evaluated_at > ?", after.UTC())
	}
	if !to.IsZero() {
		q = q.Where("e.evaluated_at < ?", to.UTC())
	}
	var rows []models.MotionEvaluationReadModel
	if err := q.Order("e.evaluated_at ASC").Limit(limit).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]entities.MotionNotice, len(rows))
	for i, r := range rows {
		out[i] = mappers.ToMotionNotice(r)
	}
	return out, nil
}
