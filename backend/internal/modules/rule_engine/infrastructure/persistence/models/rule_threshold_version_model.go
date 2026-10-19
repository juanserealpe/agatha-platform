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

	ValidFrom time.Time  `gorm:"column:valid_from;type:timestamptz;not null;index:ix_rule_threshold_versions_effective,priority:3,sort:desc"`
	ValidTo   *time.Time `gorm:"column:valid_to;type:timestamptz;check:chk_rule_threshold_versions_validity,valid_to IS NULL OR valid_to > valid_from"`

	Reason    string    `gorm:"column:reason;type:text;not null"`
	CreatedBy string    `gorm:"column:created_by;type:text;not null"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamptz;not null;default:now()"`
}

func (RuleThresholdVersionModel) TableName() string {
	return "rule_threshold_versions"
}
