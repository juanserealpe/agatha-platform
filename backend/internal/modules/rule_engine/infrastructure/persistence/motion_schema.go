package persistence

import (
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/infrastructure/persistence/models"
)

// MotionExcludeConstraint impide que dos versiones de movimiento del mismo
// punto se solapen (rangos [valid_from, valid_to), NULL = infinito).
const MotionExcludeConstraint = "ex_motion_threshold_versions_no_overlap"

// migrateMotion crea motion_threshold_versions y motion_evaluations (tags de
// los modelos) y el EXCLUDE de vigencias. Idempotente. btree_gist ya lo creó Migrate.
func migrateMotion(db *gorm.DB) error {
	if err := db.AutoMigrate(&models.MotionThresholdVersionModel{}, &models.MotionEvaluationModel{}); err != nil {
		return fmt.Errorf("automigrate movimiento del rule_engine: %w", err)
	}
	err := db.Exec(`
DO $$
BEGIN
	IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = '` + MotionExcludeConstraint + `') THEN
		ALTER TABLE motion_threshold_versions
			ADD CONSTRAINT ` + MotionExcludeConstraint + `
			EXCLUDE USING gist (
				measuring_point_id WITH =,
				tstzrange(valid_from, valid_to) WITH &&
			);
	END IF;
END $$;`).Error
	if err != nil {
		return fmt.Errorf("crear %s: %w", MotionExcludeConstraint, err)
	}
	return nil
}

// SeedInitialMotionThresholds crea, UNA sola vez (solo si
// motion_threshold_versions está vacía), la v1 de cada punto de devices con
// valid_from = su primera medición de pitch o roll (now() si no tiene). El
// MP-11556 recibe los valores de §5.2; el resto, la de punto nuevo
// (alerts_enabled = false, SOLO_MOVIMIENTO). Devuelve cuántas creó.
func SeedInitialMotionThresholds(db *gorm.DB) (int64, error) {
	migrator := db.Migrator()
	if !migrator.HasTable("devices") || !migrator.HasTable("raw_measurements") {
		return 0, nil
	}

	var created int64
	err := db.Transaction(func(tx *gorm.DB) error {
		var existing int64
		if err := tx.Model(&models.MotionThresholdVersionModel{}).Count(&existing).Error; err != nil {
			return err
		}
		if existing > 0 {
			return nil
		}
		for _, seed := range []struct {
			params entities.MotionParams
			where  string
		}{
			{entities.DefaultMotionParams(), "d.measuring_point_id = ?"},
			{entities.NewPointMotionParams(), "d.measuring_point_id <> ?"},
		} {
			sql, args := motionSeedInsert(seed.params, seed.where)
			result := tx.Exec(sql, args...)
			if result.Error != nil {
				return result.Error
			}
			created += result.RowsAffected
		}
		return nil
	})
	return created, err
}

// motionSeedInsert arma el INSERT ... SELECT de la v1 con las columnas de
// entities.MotionParamFields (los nombres de columna son los de la API).
func motionSeedInsert(p entities.MotionParams, where string) (string, []any) {
	fields := entities.MotionParamFields()
	cols := make([]string, len(fields))
	marks := make([]string, len(fields))
	args := make([]any, 0, len(fields)+3)
	for i, f := range fields {
		cols[i] = f.Name
		marks[i] = "?"
		v := f.Get(p)
		if m, ok := v.(entities.TempMode); ok {
			v = string(m)
		}
		args = append(args, v)
	}
	args = append(args, entities.SeedReason, entities.SystemCreatedBy, entities.MotionPilotPointID)

	sql := `
INSERT INTO motion_threshold_versions (measuring_point_id, version, ` + strings.Join(cols, ", ") + `, valid_from, valid_to, reason, created_by)
SELECT d.measuring_point_id, 1, ` + strings.Join(marks, ", ") + `,
	COALESCE((SELECT MIN(r.timestamp) FROM raw_measurements r
		WHERE r.measuring_point_id = d.measuring_point_id AND r.metric_id IN (1, 2)), now()),
	NULL, ?, ?
FROM (SELECT DISTINCT measuring_point_id FROM devices) d
WHERE ` + where + `
ON CONFLICT DO NOTHING`
	return sql, args
}
