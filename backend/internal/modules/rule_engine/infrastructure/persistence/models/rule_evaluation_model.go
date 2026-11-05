package models

import "time"

// RuleEvaluationModel es una fila de rule_evaluations: una por (punto, evento,
// hora calculada). AutoMigrate crea la tabla, los CHECK de los enums, el
// UNIQUE de idempotencia, el índice de "últimas K horas" y la FK a la versión
// de umbrales usada. La fila tiene todo lo que lleva su level_evaluated (más
// la versión de umbrales por su FK).
type RuleEvaluationModel struct {
	ID               int64     `gorm:"column:id;primaryKey;autoIncrement"`
	MeasuringPointID int64     `gorm:"column:measuring_point_id;not null;uniqueIndex:ux_rule_evaluations_point_event_hour,priority:1;index:ix_rule_evaluations_point_event_recent,priority:1"`
	EventType        string    `gorm:"column:event_type;type:text;not null;default:'FUGA';uniqueIndex:ux_rule_evaluations_point_event_hour,priority:2;index:ix_rule_evaluations_point_event_recent,priority:2"`
	EvaluatedAt      time.Time `gorm:"column:evaluated_at;type:timestamptz;not null;uniqueIndex:ux_rule_evaluations_point_event_hour,priority:3;index:ix_rule_evaluations_point_event_recent,priority:3,sort:desc"`

	IndicatorValue float64 `gorm:"column:indicator_value;type:numeric;not null"`
	RawLevel       string  `gorm:"column:raw_level;type:text;not null;check:chk_rule_evaluations_raw_level,raw_level IN ('NORMAL', 'AMARILLO', 'NARANJA', 'ROJO')"`
	ComputedLevel  string  `gorm:"column:computed_level;type:text;not null;check:chk_rule_evaluations_computed_level,computed_level IN ('NORMAL', 'AMARILLO', 'NARANJA', 'ROJO')"`
	Level          string  `gorm:"column:level;type:text;not null;check:chk_rule_evaluations_level,level IN ('NORMAL', 'AMARILLO', 'NARANJA', 'ROJO')"`
	Rule           string  `gorm:"column:rule;type:text;not null;check:chk_rule_evaluations_rule,rule IN ('THRESHOLD', 'PERSISTENCE', 'HOLD', 'NONE')"`

	InEpisode    bool `gorm:"column:in_episode;not null"`
	HoursAbove   int  `gorm:"column:hours_above;type:integer;not null"`
	HoursInLevel int  `gorm:"column:hours_in_level;type:integer;not null"`

	// v3: rachas, racha de salida y d_pl con los que se interpretó la hora
	// (las filas anteriores a la columna quedan en 0 / NULL; -rebuild las recalcula).
	RunAmarillo int      `gorm:"column:run_amarillo;type:integer;not null;default:0"`
	RunNaranja  int      `gorm:"column:run_naranja;type:integer;not null;default:0"`
	BelowRun    int      `gorm:"column:below_run;type:integer;not null;default:0"`
	DPl         *float64 `gorm:"column:d_pl;type:double precision"`

	ThresholdVersionID int64                     `gorm:"column:threshold_version_id;not null"`
	ThresholdVersion   RuleThresholdVersionModel `gorm:"foreignKey:ThresholdVersionID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:RESTRICT"`

	LowConfidence bool   `gorm:"column:low_confidence;not null"`
	EpisodeEvent  string `gorm:"column:episode_event;type:text;not null;check:chk_rule_evaluations_episode_event,episode_event IN ('NONE', 'OPENED', 'CLEARED')"`
	// Reasons es un arreglo JSON de ReasonRow.
	Reasons   string    `gorm:"column:reasons;type:jsonb;not null;default:'[]'"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamptz;not null;default:now()"`
}

// ReasonRow es un elemento de rule_evaluations.reasons (mismo formato que
// contract.Reason).
type ReasonRow struct {
	Code  string        `json:"code"`
	Level string        `json:"level"`
	Data  ReasonDataRow `json:"data"`
}

type ReasonDataRow struct {
	I           float64  `json:"i,omitempty"`
	Threshold   float64  `json:"threshold,omitempty"`
	Hours       int      `json:"hours,omitempty"`
	From        string   `json:"from,omitempty"`
	WindowHours int      `json:"window_hours,omitempty"`
	DPl         *float64 `json:"d_pl,omitempty"`
	DPlMin      float64  `json:"d_pl_min,omitempty"`
}

func (RuleEvaluationModel) TableName() string {
	return "rule_evaluations"
}
