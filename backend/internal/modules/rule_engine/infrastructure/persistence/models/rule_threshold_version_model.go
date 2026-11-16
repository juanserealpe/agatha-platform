// Package models son las filas de las tablas del Rule Engine (escritura) y el
// modelo de solo lectura de feature_evaluations (de Processing, no se migra).
package models

import "time"

// RuleThresholdVersionModel es una versión de umbrales. AutoMigrate crea la
// tabla, los CHECK, el UNIQUE (punto, evento, versión) y el índice de "vigente
// en t". El EXCLUDE de solapamientos (btree_gist) no se puede expresar con
// tags: lo agrega persistence.Migrate con SQL idempotente.
// measuring_point_id no tiene FK: la tabla devices es de Ingestion.
type RuleThresholdVersionModel struct {
	ID               int64  `gorm:"column:id;primaryKey;autoIncrement"`
	MeasuringPointID int64  `gorm:"column:measuring_point_id;not null;uniqueIndex:ux_rule_threshold_versions_point_event_version,priority:1;index:ix_rule_threshold_versions_effective,priority:1"`
	EventType        string `gorm:"column:event_type;type:text;not null;default:'FUGA';uniqueIndex:ux_rule_threshold_versions_point_event_version,priority:2;index:ix_rule_threshold_versions_effective,priority:2"`
	Version          int    `gorm:"column:version;type:integer;not null;uniqueIndex:ux_rule_threshold_versions_point_event_version,priority:3"`

	Amarillo float64 `gorm:"column:amarillo;type:numeric(5,4);not null;check:chk_rule_threshold_versions_order,amarillo > 0 AND amarillo < naranja AND naranja < rojo AND rojo <= 1"`
	Naranja  float64 `gorm:"column:naranja;type:numeric(5,4);not null"`
	Rojo     float64 `gorm:"column:rojo;type:numeric(5,4);not null"`

	MinHours                   int  `gorm:"column:min_hours;type:integer;not null;check:chk_rule_threshold_versions_min_hours,min_hours >= 1"`
	EscalateAmarilloAfterHours *int `gorm:"column:escalate_amarillo_after_hours;type:integer;check:chk_rule_threshold_versions_escalate_amarillo,escalate_amarillo_after_hours IS NULL OR escalate_amarillo_after_hours >= 1"`
	EscalateNaranjaAfterHours  *int `gorm:"column:escalate_naranja_after_hours;type:integer;check:chk_rule_threshold_versions_escalate_naranja,escalate_naranja_after_hours IS NULL OR escalate_naranja_after_hours >= 1"`

	ExitMargin float64 `gorm:"column:exit_margin;type:numeric(5,4);not null;check:chk_rule_threshold_versions_exit_margin,exit_margin >= 0 AND exit_margin < amarillo"`
	ExitHours  int     `gorm:"column:exit_hours;type:integer;not null;check:chk_rule_threshold_versions_exit_hours,exit_hours >= 1"`

	// Columnas v2 (spec Rule Engine v2, §7): los defaults reproducen el
	// comportamiento anterior, así que las versiones existentes no cambian.
	ConfirmNaranjaWithPlanitud bool `gorm:"column:confirm_naranja_with_planitud;not null;default:false;check:chk_rule_threshold_versions_planitud,NOT confirm_naranja_with_planitud OR d_pl_min IS NOT NULL"`
	// ConfirmRojoWithPlanitud (v4): ROJO también exige planitud. El default false
	// deja las versiones existentes idénticas. El CHECK ata la opción a la de NARANJA.
	ConfirmRojoWithPlanitud bool     `gorm:"column:confirm_rojo_with_planitud;not null;default:false;check:chk_rule_threshold_versions_planitud_rojo,NOT confirm_rojo_with_planitud OR confirm_naranja_with_planitud"`
	DPlMin                  *float64 `gorm:"column:d_pl_min;type:numeric(5,2);check:chk_rule_threshold_versions_d_pl_min,d_pl_min IS NULL OR (d_pl_min >= 0 AND d_pl_min <= 100)"`
	PersistenceMode         string   `gorm:"column:persistence_mode;type:text;not null;default:'STRICT';check:chk_rule_threshold_versions_persistence,persistence_mode = 'STRICT' OR (persistence_mode = 'WINDOW' AND persistence_window_hours IS NOT NULL AND persistence_min_hours IS NOT NULL AND persistence_min_hours <= persistence_window_hours)"`
	PersistenceWindowHours  *int     `gorm:"column:persistence_window_hours;type:integer;check:chk_rule_threshold_versions_window_hours,persistence_window_hours IS NULL OR persistence_window_hours >= 1"`
	PersistenceMinHours     *int     `gorm:"column:persistence_min_hours;type:integer;check:chk_rule_threshold_versions_window_min_hours,persistence_min_hours IS NULL OR persistence_min_hours >= 1"`

	ValidFrom time.Time  `gorm:"column:valid_from;type:timestamptz;not null;index:ix_rule_threshold_versions_effective,priority:3,sort:desc"`
	ValidTo   *time.Time `gorm:"column:valid_to;type:timestamptz;check:chk_rule_threshold_versions_validity,valid_to IS NULL OR valid_to > valid_from"`

	Reason    string    `gorm:"column:reason;type:text;not null"`
	CreatedBy string    `gorm:"column:created_by;type:text;not null"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamptz;not null;default:now()"`
}

func (RuleThresholdVersionModel) TableName() string {
	return "rule_threshold_versions"
}
