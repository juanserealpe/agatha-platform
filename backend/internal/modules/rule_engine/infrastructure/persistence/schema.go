// Package persistence agrupa el esquema del Rule Engine: la migración
// (AutoMigrate + SQL idempotente para lo que los tags de GORM no expresan) y
// el seed de la versión 1 de umbrales.
package persistence

import (
	"fmt"

	"gorm.io/gorm"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/domain/entities"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/rule_engine/infrastructure/persistence/models"
)

// ExcludeConstraint impide que dos versiones del mismo punto y evento se
// solapen en el tiempo (rangos [valid_from, valid_to), NULL = infinito).
const ExcludeConstraint = "ex_rule_threshold_versions_no_overlap"

// Migrate crea/actualiza el esquema del Rule Engine. Es idempotente: corre en
// cada arranque, como el AutoMigrate de los demás módulos.
//
//  0. upgradeRuleEvaluationsToV2: convierte una rule_evaluations del esquema v1
//     (columna emitted) antes de que AutoMigrate vea las columnas NOT NULL nuevas.
//  1. AutoMigrate: tablas, CHECK, UNIQUE, índices y FK (tags de los modelos).
//  2. dropPlanitudColumns: retira la confirmación por planitud (nov 2026).
//  3. SQL idempotente: btree_gist + EXCLUDE USING gist (no expresable con tags).
func Migrate(db *gorm.DB) error {
	if err := upgradeRuleEvaluationsToV2(db); err != nil {
		return err
	}

	if err := db.AutoMigrate(&models.RuleThresholdVersionModel{}, &models.RuleEvaluationModel{}); err != nil {
		return fmt.Errorf("automigrate rule_engine: %w", err)
	}

	if err := dropPlanitudColumns(db); err != nil {
		return err
	}

	if err := db.Exec(`CREATE EXTENSION IF NOT EXISTS btree_gist`).Error; err != nil {
		return fmt.Errorf("crear la extensión btree_gist: %w", err)
	}

	err := db.Exec(`
DO $$
BEGIN
	IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = '` + ExcludeConstraint + `') THEN
		ALTER TABLE rule_threshold_versions
			ADD CONSTRAINT ` + ExcludeConstraint + `
			EXCLUDE USING gist (
				measuring_point_id WITH =,
				event_type WITH =,
				tstzrange(valid_from, valid_to) WITH &&
			);
	END IF;
END $$;`).Error
	if err != nil {
		return fmt.Errorf("crear %s: %w", ExcludeConstraint, err)
	}

	return migrateMotion(db)
}

// dropPlanitudColumns retira, de forma idempotente, todo lo que sostenía la
// confirmación por planitud (eliminada en nov 2026): los CHECK que la ataban,
// las columnas de la versión de umbrales y el d_pl guardado por hora. AutoMigrate
// ya no las conoce (se quitaron de los modelos), así que no las recrea.
// Destructivo: no hay vuelta atrás sin restaurar un respaldo.
func dropPlanitudColumns(db *gorm.DB) error {
	err := db.Exec(`
ALTER TABLE rule_threshold_versions
	DROP CONSTRAINT IF EXISTS chk_rule_threshold_versions_planitud,
	DROP CONSTRAINT IF EXISTS chk_rule_threshold_versions_planitud_rojo,
	DROP CONSTRAINT IF EXISTS chk_rule_threshold_versions_d_pl_min;
ALTER TABLE rule_threshold_versions
	DROP COLUMN IF EXISTS confirm_naranja_with_planitud,
	DROP COLUMN IF EXISTS confirm_rojo_with_planitud,
	DROP COLUMN IF EXISTS d_pl_min;
ALTER TABLE rule_evaluations
	DROP COLUMN IF EXISTS d_pl;`).Error
	if err != nil {
		return fmt.Errorf("retirar las columnas de planitud: %w", err)
	}
	return nil
}

// upgradeRuleEvaluationsToV2 pasa rule_evaluations del contrato v1
// (emitted NONE|EXCEEDED|CLEARED) al v2 (episode_event OPENED|CLEARED|NONE +
// computed_level + reasons), en UNA transacción y solo si existe la columna
// emitted (no hace nada en una BD nueva ni en una ya convertida):
//
//   - episode_event: CLEARED se conserva; EXCEEDED es OPENED si la hora
//     calculada anterior del punto no estaba en episodio; el resto, NONE.
//   - computed_level: el level si la regla fue THRESHOLD/PERSISTENCE (dentro
//     de episodio level = max(raw, persistencia)); si no, el raw (HOLD y cierre
//     tienen raw NORMAL; la racha de entrada pendiente es más corta que
//     cualquier escalamiento con min_hours <= escalate_*).
//   - reasons: '[]' (las filas viejas no guardaron la racha). Para tenerlas,
//     -rebuild fuera de producción.
func upgradeRuleEvaluationsToV2(db *gorm.DB) error {
	migrator := db.Migrator()
	if !migrator.HasTable(&models.RuleEvaluationModel{}) || !migrator.HasColumn(&models.RuleEvaluationModel{}, "emitted") {
		return nil
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		for _, stmt := range []string{
			`ALTER TABLE rule_evaluations ADD COLUMN IF NOT EXISTS computed_level text`,
			`ALTER TABLE rule_evaluations ADD COLUMN IF NOT EXISTS episode_event text`,
			`ALTER TABLE rule_evaluations ADD COLUMN IF NOT EXISTS reasons jsonb NOT NULL DEFAULT '[]'`,
			`UPDATE rule_evaluations r SET
	computed_level = CASE WHEN r.rule IN ('THRESHOLD', 'PERSISTENCE') THEN r.level ELSE r.raw_level END,
	episode_event = CASE
		WHEN r.emitted = 'CLEARED' THEN 'CLEARED'
		WHEN r.emitted = 'EXCEEDED' AND NOT COALESCE(p.prev_in_episode, false) THEN 'OPENED'
		ELSE 'NONE'
	END
FROM (
	SELECT id, LAG(in_episode) OVER (PARTITION BY measuring_point_id, event_type ORDER BY evaluated_at) AS prev_in_episode
	FROM rule_evaluations
) p
WHERE p.id = r.id`,
			`ALTER TABLE rule_evaluations ALTER COLUMN computed_level SET NOT NULL, ALTER COLUMN episode_event SET NOT NULL`,
			`ALTER TABLE rule_evaluations DROP CONSTRAINT IF EXISTS chk_rule_evaluations_emitted`,
			`ALTER TABLE rule_evaluations DROP COLUMN emitted`,
		} {
			if err := tx.Exec(stmt).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("convertir rule_evaluations al esquema v2: %w", err)
	}
	return nil
}

// SeedInitialThresholds crea, UNA sola vez (solo si rule_threshold_versions
// está vacía), la versión 1 de cada punto de devices con la plantilla por
// defecto y valid_from = su primera medición en raw_measurements (now() si no
// tiene ninguna).
//
// Es la ÚNICA excepción a la no retroactividad: la versión inicial cubre el
// histórico para que el backfill pueda interpretarlo. Un punto que aparezca
// después recibe su versión 1 en el primer uso con valid_from = now.
//
// Devuelve cuántas versiones creó. Sin la tabla devices (Ingestion nunca
// migró) no hace nada.
func SeedInitialThresholds(db *gorm.DB) (int64, error) {
	migrator := db.Migrator()
	if !migrator.HasTable("devices") || !migrator.HasTable("raw_measurements") {
		return 0, nil
	}

	p := entities.DefaultThresholdParams()
	result := db.Exec(`
INSERT INTO rule_threshold_versions (
	measuring_point_id, event_type, version,
	amarillo, naranja, rojo, min_hours,
	escalate_amarillo_after_hours, escalate_naranja_after_hours,
	exit_margin, exit_hours, valid_from, valid_to, reason, created_by
)
SELECT d.measuring_point_id, ?, 1,
	?, ?, ?, ?,
	?, ?,
	?, ?,
	COALESCE((SELECT MIN(r.timestamp) FROM raw_measurements r WHERE r.measuring_point_id = d.measuring_point_id), now()),
	NULL, ?, ?
FROM devices d
WHERE NOT EXISTS (SELECT 1 FROM rule_threshold_versions)
ON CONFLICT DO NOTHING`,
		string(entities.EventTypeLeak),
		p.Amarillo, p.Naranja, p.Rojo, p.MinHours,
		p.EscalateAmarilloAfterHours, p.EscalateNaranjaAfterHours,
		p.ExitMargin, p.ExitHours,
		entities.SeedReason, entities.SystemCreatedBy,
	)
	return result.RowsAffected, result.Error
}
