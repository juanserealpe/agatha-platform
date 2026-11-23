package models

import "time"

// ExclusionPeriodModel es una fila de baseline_exclusion_periods (de Baseline,
// la migra NewBaselineModule). Un período de ALERT se identifica por
// source_alert_code (UNIQUE; NULL en los MANUAL). Sin FK a alerts ni a
// devices: es un dato de entrada que sobrevive a cualquier rebuild.
type ExclusionPeriodModel struct {
	ID               int64      `gorm:"column:id;primaryKey"`
	MeasuringPointID int64      `gorm:"column:measuring_point_id;not null;index:ix_baseline_exclusion_periods_point_start,priority:1"`
	StartedAt        time.Time  `gorm:"column:started_at;not null;index:ix_baseline_exclusion_periods_point_start,priority:2"`
	EndedAt          *time.Time `gorm:"column:ended_at;check:chk_baseline_exclusion_periods_range,ended_at IS NULL OR ended_at >= started_at"`
	Source           string     `gorm:"column:source;type:text;not null;check:chk_baseline_exclusion_periods_source,(source = 'ALERT' AND source_alert_code IS NOT NULL) OR (source = 'MANUAL' AND source_alert_code IS NULL)"`
	SourceAlertCode  *string    `gorm:"column:source_alert_code;type:text;uniqueIndex:ux_baseline_exclusion_periods_alert_code"`
	Note             string     `gorm:"column:note;type:text;not null;default:''"`
	CreatedAt        time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt        time.Time  `gorm:"column:updated_at;not null"`
}

func (ExclusionPeriodModel) TableName() string {
	return "baseline_exclusion_periods"
}

// AlertSeedRowModel es la vista propia (solo lectura) de alerts para copiar
// sus intervalos. La tabla es de Alert Management: NO se migra acá.
type AlertSeedRowModel struct {
	Code             string     `gorm:"column:code"`
	MeasuringPointID int64      `gorm:"column:measuring_point_id"`
	StartedAt        time.Time  `gorm:"column:started_at"`
	EndedAt          *time.Time `gorm:"column:ended_at"`
}
