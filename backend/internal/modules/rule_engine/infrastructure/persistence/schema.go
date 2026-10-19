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
//  1. AutoMigrate: tablas, CHECK, UNIQUE, índices y FK (tags de los modelos).
//  2. SQL idempotente: btree_gist + EXCLUDE USING gist (no expresable con tags).
func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(&models.RuleThresholdVersionModel{}, &models.RuleEvaluationModel{}); err != nil {
		return fmt.Errorf("automigrate rule_engine: %w", err)
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
