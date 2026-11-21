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

// motionV1Checks son los CHECK de motion-v1 que motion-v2 amplía: se borran
// antes del AutoMigrate, que los vuelve a crear con la definición nueva.
var motionV1Checks = []string{"chk_motion_evaluations_computed_level"}

// migrateMotion crea motion_threshold_versions, motion_threshold_rules y
// motion_evaluations (tags de los modelos), el EXCLUDE de vigencias y siembra
// la tabla de §5.3 en las versiones sin filas. Idempotente y aditiva.
// btree_gist ya lo creó Migrate.
func migrateMotion(db *gorm.DB) error {
	for _, chk := range motionV1Checks {
		if err := db.Exec(`ALTER TABLE IF EXISTS motion_evaluations DROP CONSTRAINT IF EXISTS ` + chk).Error; err != nil {
			return fmt.Errorf("borrar %s: %w", chk, err)
		}
	}
	if err := db.AutoMigrate(&models.MotionThresholdVersionModel{}, &models.MotionThresholdRuleModel{}, &models.MotionEvaluationModel{}); err != nil {
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
	if _, err := SeedMissingMotionRules(db); err != nil {
		return fmt.Errorf("sembrar motion_threshold_rules: %w", err)
	}
	return nil
}

// SeedMissingMotionRules siembra la tabla de §5.3 en cada versión de
// movimiento que no tenga filas (al migrar, las versiones motion-v1; hoy, la
// id 1). Es la misma excepción deliberada a la no retroactividad que el seed:
// sin ella, el backfill no tendría tabla para el histórico. Devuelve cuántas
// filas insertó.
func SeedMissingMotionRules(db *gorm.DB) (int64, error) {
	rules := entities.DefaultMotionRules()
	values := make([]string, len(rules))
	args := make([]any, 0, len(rules)*5)
	for i, r := range rules {
		values[i] = "(?::integer, ?::text, ?::numeric, ?::numeric, ?::numeric)"
		args = append(args, r.WindowHours, r.Axis, r.AmarilloDeg, r.NaranjaDeg, r.RojoDeg)
	}
	result := db.Exec(`
INSERT INTO motion_threshold_rules (threshold_version_id, window_hours, axis, amarillo_deg, naranja_deg, rojo_deg)
SELECT v.id, r.w, r.a, r.am, r.na, r.ro
FROM motion_threshold_versions v
CROSS JOIN (VALUES `+strings.Join(values, ", ")+`) AS r(w, a, am, na, ro)
WHERE NOT EXISTS (SELECT 1 FROM motion_threshold_rules x WHERE x.threshold_version_id = v.id)`, args...)
	return result.RowsAffected, result.Error
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
		_, err := SeedMissingMotionRules(tx)
		return err
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
		args = append(args, f.GetString(p))
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
