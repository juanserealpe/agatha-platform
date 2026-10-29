package models

import "time"

// BaselineVersionReadModel es la vista de SOLO LECTURA de Processing sobre
// baseline_versions (tabla del módulo Baseline). Trae únicamente las columnas
// que el cálculo necesita. NUNCA se pasa a AutoMigrate: el esquema es de
// Baseline. Copia deliberada: los módulos no comparten modelos.
type BaselineVersionReadModel struct {
	ID               int64      `gorm:"column:id;primaryKey"`
	MeasuringPointID int64      `gorm:"column:measuring_point_id"`
	Version          int        `gorm:"column:version"`
	ValidFrom        time.Time  `gorm:"column:valid_from"`
	ValidTo          *time.Time `gorm:"column:valid_to"`
	MedianB2         float64    `gorm:"column:median_b2"`
	ThetaD           float64    `gorm:"column:theta_d"`
	P90Planitud      *float64   `gorm:"column:p90_planitud"`
	MinElevatedB2    *float64   `gorm:"column:min_elevated_b2"`
	DeltaMinB2       float64    `gorm:"column:delta_min_b2"`
	Confidence       string     `gorm:"column:confidence"`
	Flags            TextArray  `gorm:"column:flags;type:text[]"`
	AlgorithmVersion string     `gorm:"column:algorithm_version"`

	ComponentStats []BaselineComponentStatReadModel `gorm:"foreignKey:BaselineVersionID;references:ID"`
}

func (BaselineVersionReadModel) TableName() string {
	return "baseline_versions"
}

// BaselineComponentStatReadModel es la vista de solo lectura de
// baseline_component_stats. MAD crudo (sin 1,4826 ni piso).
type BaselineComponentStatReadModel struct {
	BaselineVersionID int64   `gorm:"column:baseline_version_id;primaryKey"`
	WindowHours       int16   `gorm:"column:window_hours;primaryKey"`
	Component         string  `gorm:"column:component;primaryKey"`
	Median            float64 `gorm:"column:median"`
	MAD               float64 `gorm:"column:mad"`
	NWindows          int     `gorm:"column:n_windows"`
}

func (BaselineComponentStatReadModel) TableName() string {
	return "baseline_component_stats"
}
