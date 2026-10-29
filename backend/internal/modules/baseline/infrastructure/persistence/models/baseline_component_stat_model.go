package models

// BaselineComponentStatModel es la estadística de un componente de una versión.
// MAD crudo: sin 1,4826 ni piso (los pisos se derivan de la señal y del kernel).
type BaselineComponentStatModel struct {
	BaselineVersionID int64   `gorm:"column:baseline_version_id;primaryKey;autoIncrement:false"`
	WindowHours       int16   `gorm:"column:window_hours;type:smallint;primaryKey;autoIncrement:false;check:chk_baseline_component_stats_window_hours,window_hours IN (6, 24)"`
	Component         string  `gorm:"column:component;type:varchar(4);primaryKey;check:chk_baseline_component_stats_component,component IN ('D', 'P', 'EX', 'C')"`
	Median            float64 `gorm:"column:median;type:double precision;not null"`
	MAD               float64 `gorm:"column:mad;type:double precision;not null"`
	NWindows          int     `gorm:"column:n_windows;not null"`
}

func (BaselineComponentStatModel) TableName() string {
	return "baseline_component_stats"
}
