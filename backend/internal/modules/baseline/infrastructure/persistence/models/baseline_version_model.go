package models

import "time"

// BaselineVersionModel es una versión de línea base. El esquema lo crea
// AutoMigrate, incluidos el índice parcial de "una sola vigente por punto"
// (ux_baseline_versions_current) y el índice de vigencia. measuring_point_id
// no tiene FK: la tabla de dispositivos es de Ingestion.
type BaselineVersionModel struct {
	ID                 int64      `gorm:"column:id;primaryKey;autoIncrement"`
	MeasuringPointID   int64      `gorm:"column:measuring_point_id;not null;uniqueIndex:ux_baseline_versions_point_version,priority:1;uniqueIndex:ux_baseline_versions_current,where:valid_to IS NULL;index:ix_baseline_versions_effective,priority:1"`
	Version            int        `gorm:"column:version;not null;uniqueIndex:ux_baseline_versions_point_version,priority:2"`
	ValidFrom          time.Time  `gorm:"column:valid_from;type:timestamptz;not null;index:ix_baseline_versions_effective,priority:2,sort:desc"`
	ValidTo            *time.Time `gorm:"column:valid_to;type:timestamptz"`
	HistoryFrom        time.Time  `gorm:"column:history_from;type:timestamptz;not null"`
	HistoryTo          time.Time  `gorm:"column:history_to;type:timestamptz;not null"`
	MedianB2           float64    `gorm:"column:median_b2;type:double precision;not null"`
	ThetaD             float64    `gorm:"column:theta_d;type:double precision;not null"`
	P90Planitud        *float64   `gorm:"column:p90_planitud;type:double precision"`
	MinElevatedB2      *float64   `gorm:"column:min_elevated_b2;type:double precision"`
	DeltaMinB2         float64    `gorm:"column:delta_min_b2;type:double precision;not null"`
	NSignalRecords     int        `gorm:"column:n_signal_records;not null"`
	NPlanitudRecords   int        `gorm:"column:n_planitud_records;not null"`
	Confidence         string     `gorm:"column:confidence;type:varchar(8);not null"`
	ExclusionPasses    int16      `gorm:"column:exclusion_passes;type:smallint;not null"`
	ExclusionConverged bool       `gorm:"column:exclusion_converged;not null"`
	ExcludedRatio      float64    `gorm:"column:excluded_ratio;type:double precision;not null"`
	ExcludedPeriods    string     `gorm:"column:excluded_periods;type:jsonb;not null"`
	Flags              TextArray  `gorm:"column:flags;type:text[];not null;default:'{}'"`
	Trigger            string     `gorm:"column:trigger;type:varchar(16);not null"`
	AlgorithmVersion   string     `gorm:"column:algorithm_version;type:varchar(16);not null"`
	CreatedAt          time.Time  `gorm:"column:created_at;type:timestamptz;not null;default:now()"`

	ComponentStats []BaselineComponentStatModel `gorm:"foreignKey:BaselineVersionID;references:ID"`
}

func (BaselineVersionModel) TableName() string {
	return "baseline_versions"
}

// ExcludedPeriodJSON es el formato de cada elemento de excluded_periods: solo
// fechas. Las filas viejas traen además peak_i, que se ignora al leer.
type ExcludedPeriodJSON struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}
