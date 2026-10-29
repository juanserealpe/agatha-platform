package models

import "time"

// FeatureEvaluationReadModel es la vista de SOLO LECTURA del Rule Engine sobre
// feature_evaluations (la escribe Processing). NO se migra: el acople es de
// esquema, igual que Processing con las tablas de Baseline.
type FeatureEvaluationReadModel struct {
	MeasuringPointID int64     `gorm:"column:measuring_point_id"`
	WindowHours      int16     `gorm:"column:window_hours"`
	EvaluatedAt      time.Time `gorm:"column:evaluated_at"`
	Status           string    `gorm:"column:status"`
	I                *float64  `gorm:"column:i"`
	DPl              *float64  `gorm:"column:d_pl"`
	LowConfidence    bool      `gorm:"column:low_confidence"`
}

func (FeatureEvaluationReadModel) TableName() string {
	return "feature_evaluations"
}
