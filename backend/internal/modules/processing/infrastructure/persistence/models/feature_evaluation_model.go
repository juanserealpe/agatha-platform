package models

import "time"

// FeatureEvaluationModel es una fila de feature_evaluations: una por (punto,
// ventana, hora). El esquema lo crea AutoMigrate, incluido el índice único
// ux_feature_evaluations_point_window_hour (clave de idempotencia, "última hora
// evaluada" y lectura de las últimas filas del Rule Engine). measuring_point_id
// y baseline_version_id no tienen FK: las tablas son de otros módulos.
type FeatureEvaluationModel struct {
	ID               int64     `gorm:"column:id;primaryKey;autoIncrement"`
	MeasuringPointID int64     `gorm:"column:measuring_point_id;not null;uniqueIndex:ux_feature_evaluations_point_window_hour,priority:1"`
	WindowHours      int16     `gorm:"column:window_hours;type:smallint;not null;uniqueIndex:ux_feature_evaluations_point_window_hour,priority:2;check:chk_feature_evaluations_window_hours,window_hours IN (6, 24)"`
	EvaluatedAt      time.Time `gorm:"column:evaluated_at;type:timestamptz;not null;uniqueIndex:ux_feature_evaluations_point_window_hour,priority:3"`
	WindowStart      time.Time `gorm:"column:window_start;type:timestamptz;not null"`

	Status       string  `gorm:"column:status;type:varchar(16);not null"`
	NoCalcReason *string `gorm:"column:no_calc_reason;type:varchar(40)"`

	NExpected int16   `gorm:"column:n_expected;type:smallint;not null"`
	NReceived int16   `gorm:"column:n_received;type:smallint;not null"`
	NValid    int16   `gorm:"column:n_valid;type:smallint;not null"`
	Coverage  float64 `gorm:"column:coverage;type:double precision;not null"`

	D        *float64 `gorm:"column:d;type:double precision"`
	PHours   *float64 `gorm:"column:p_hours;type:double precision"`
	Ex       *float64 `gorm:"column:ex;type:double precision"`
	C        *float64 `gorm:"column:c;type:double precision"`
	DPl      *float64 `gorm:"column:d_pl;type:double precision"`
	CFrom24h bool     `gorm:"column:c_from_24h;not null"`

	ZD  *float64 `gorm:"column:z_d;type:double precision"`
	ZP  *float64 `gorm:"column:z_p;type:double precision"`
	ZEx *float64 `gorm:"column:z_ex;type:double precision"`
	ZC  *float64 `gorm:"column:z_c;type:double precision"`
	SD  *float64 `gorm:"column:s_d;type:double precision"`
	SP  *float64 `gorm:"column:s_p;type:double precision"`
	SEx *float64 `gorm:"column:s_ex;type:double precision"`
	SC  *float64 `gorm:"column:s_c;type:double precision"`
	I   *float64 `gorm:"column:i;type:double precision"`

	LowConfidence bool      `gorm:"column:low_confidence;not null"`
	Silent        bool      `gorm:"column:silent;not null"`
	Flags         TextArray `gorm:"column:flags;type:text[];not null;default:'{}'"`

	BaselineVersionID *int64 `gorm:"column:baseline_version_id"`
	BaselineVersion   *int   `gorm:"column:baseline_version"`

	AlgorithmVersion string     `gorm:"column:algorithm_version;type:varchar(16);not null"`
	ConfigVersion    string     `gorm:"column:config_version;type:varchar(64);not null"`
	Trigger          string     `gorm:"column:trigger;type:varchar(12);not null"`
	SourceBatchSeq   *int64     `gorm:"column:source_batch_seq"`
	InputMaxTs       *time.Time `gorm:"column:input_max_ts;type:timestamptz"`
	CreatedAt        time.Time  `gorm:"column:created_at;type:timestamptz;not null;default:now()"`
}

func (FeatureEvaluationModel) TableName() string {
	return "feature_evaluations"
}
