package models

import "time"

// RuleEvaluationModel es una fila de rule_evaluations: una por (punto, evento,
// hora calculada). AutoMigrate crea la tabla, los CHECK de los enums, el
// UNIQUE de idempotencia, el índice de "últimas K horas" y la FK a la versión
// de umbrales usada.
type RuleEvaluationModel struct {
	ID               int64     `gorm:"column:id;primaryKey;autoIncrement"`
	MeasuringPointID int64     `gorm:"column:measuring_point_id;not null;uniqueIndex:ux_rule_evaluations_point_event_hour,priority:1;index:ix_rule_evaluations_point_event_recent,priority:1"`
	EventType        string    `gorm:"column:event_type;type:text;not null;default:'FUGA';uniqueIndex:ux_rule_evaluations_point_event_hour,priority:2;index:ix_rule_evaluations_point_event_recent,priority:2"`
	EvaluatedAt      time.Time `gorm:"column:evaluated_at;type:timestamptz;not null;uniqueIndex:ux_rule_evaluations_point_event_hour,priority:3;index:ix_rule_evaluations_point_event_recent,priority:3,sort:desc"`

	IndicatorValue float64 `gorm:"column:indicator_value;type:numeric;not null"`
	RawLevel       string  `gorm:"column:raw_level;type:text;not null;check:chk_rule_evaluations_raw_level,raw_level IN ('NORMAL', 'AMARILLO', 'NARANJA', 'ROJO')"`
	Level          string  `gorm:"column:level;type:text;not null;check:chk_rule_evaluations_level,level IN ('NORMAL', 'AMARILLO', 'NARANJA', 'ROJO')"`
	Rule           string  `gorm:"column:rule;type:text;not null;check:chk_rule_evaluations_rule,rule IN ('THRESHOLD', 'PERSISTENCE', 'HOLD', 'NONE')"`

	InEpisode    bool `gorm:"column:in_episode;not null"`
	HoursAbove   int  `gorm:"column:hours_above;type:integer;not null"`
	HoursInLevel int  `gorm:"column:hours_in_level;type:integer;not null"`

	ThresholdVersionID int64                     `gorm:"column:threshold_version_id;not null"`
	ThresholdVersion   RuleThresholdVersionModel `gorm:"foreignKey:ThresholdVersionID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:RESTRICT"`

	LowConfidence bool      `gorm:"column:low_confidence;not null"`
	Emitted       string    `gorm:"column:emitted;type:text;not null;check:chk_rule_evaluations_emitted,emitted IN ('NONE', 'EXCEEDED', 'CLEARED')"`
	CreatedAt     time.Time `gorm:"column:created_at;type:timestamptz;not null;default:now()"`
}

func (RuleEvaluationModel) TableName() string {
	return "rule_evaluations"
}
