package models

import "time"

// MotionThresholdVersionModel es una versión de motion_threshold_versions
// (§6.1). Los nombres de columna son los de entities.MotionParamFields. El
// EXCLUDE de solapamientos lo agrega persistence.Migrate con SQL.
type MotionThresholdVersionModel struct {
	ID               int64 `gorm:"column:id;primaryKey;autoIncrement"`
	MeasuringPointID int64 `gorm:"column:measuring_point_id;not null;uniqueIndex:ux_motion_threshold_versions_point_version,priority:1;index:ix_motion_threshold_versions_effective,priority:1"`
	Version          int   `gorm:"column:version;type:integer;not null;uniqueIndex:ux_motion_threshold_versions_point_version,priority:2"`

	AlertsEnabled bool   `gorm:"column:alerts_enabled;not null"`
	TempMode      string `gorm:"column:temp_mode;type:text;not null;check:chk_motion_threshold_versions_temp_mode,temp_mode IN ('MOVIMIENTO_Y_TEMPERATURA', 'SOLO_MOVIMIENTO')"`

	RecordsPerHour int `gorm:"column:records_per_hour;type:integer;not null;check:chk_motion_threshold_versions_records,min_records_hour >= 1 AND min_records_hour <= records_per_hour"`
	MinRecordsHour int `gorm:"column:min_records_hour;type:integer;not null"`

	JumpDeg       float64 `gorm:"column:jump_deg;type:numeric(6,4);not null;check:chk_motion_threshold_versions_jump,jump_deg > 0 AND jump_deg <= shift_deg"`
	DerivaJumpDeg float64 `gorm:"column:deriva_jump_deg;type:numeric(6,4);not null;check:chk_motion_threshold_versions_deriva_jump,deriva_jump_deg > 0"`

	ShiftDeg         float64 `gorm:"column:shift_deg;type:numeric(6,4);not null"`
	ShiftWindowHours int     `gorm:"column:shift_window_hours;type:integer;not null"`
	ShiftMinRecords  int     `gorm:"column:shift_min_records;type:integer;not null"`

	BaseFromHours  int `gorm:"column:base_from_hours;type:integer;not null"`
	BaseToHours    int `gorm:"column:base_to_hours;type:integer;not null"`
	BaseMinRecords int `gorm:"column:base_min_records;type:integer;not null"`

	GapMinHours       int `gorm:"column:gap_min_hours;type:integer;not null"`
	GapPreHours       int `gorm:"column:gap_pre_hours;type:integer;not null"`
	GapPostMinutes    int `gorm:"column:gap_post_minutes;type:integer;not null"`
	GapPreMinRecords  int `gorm:"column:gap_pre_min_records;type:integer;not null"`
	GapPostMinRecords int `gorm:"column:gap_post_min_records;type:integer;not null"`

	ReturnFraction                float64 `gorm:"column:return_fraction;type:numeric(6,4);not null;check:chk_motion_threshold_versions_return_fraction,return_fraction > 0 AND return_fraction <= 1"`
	ShiftSuppressAfterReturnHours int     `gorm:"column:shift_suppress_after_return_hours;type:integer;not null"`

	AbruptDeg                     float64 `gorm:"column:abrupt_deg;type:numeric(6,4);not null;check:chk_motion_threshold_versions_abrupt,abrupt_deg > 0"`
	AbruptHours                   int     `gorm:"column:abrupt_hours;type:integer;not null"`
	AbruptSuppressAfterJointHours int     `gorm:"column:abrupt_suppress_after_joint_hours;type:integer;not null"`
	SlowDriftDeg24h               float64 `gorm:"column:slow_drift_deg_24h;type:numeric(6,4);not null"`

	StillHours         int     `gorm:"column:still_hours;type:integer;not null"`
	StillMinCalculated int     `gorm:"column:still_min_calculated;type:integer;not null"`
	StillRangeDeg      float64 `gorm:"column:still_range_deg;type:numeric(6,4);not null"`

	TempFlatRate     float64 `gorm:"column:temp_flat_rate;type:numeric(6,4);not null"`
	TempFlatHours    int     `gorm:"column:temp_flat_hours;type:integer;not null"`
	TempFlatMinRates int     `gorm:"column:temp_flat_min_rates;type:integer;not null"`
	TempFlowRate     float64 `gorm:"column:temp_flow_rate;type:numeric(6,4);not null"`
	TempFlowHours    int     `gorm:"column:temp_flow_hours;type:integer;not null"`

	StabilizeMaxHours     int `gorm:"column:stabilize_max_hours;type:integer;not null"`
	StillStableFirstHours int `gorm:"column:still_stable_first_hours;type:integer;not null"`
	StillStableEveryHours int `gorm:"column:still_stable_every_hours;type:integer;not null;check:chk_motion_threshold_versions_hours,shift_window_hours >= 1 AND shift_min_records >= 1 AND base_from_hours >= 1 AND base_to_hours >= 1 AND base_min_records >= 1 AND gap_min_hours >= 1 AND gap_pre_hours >= 1 AND gap_post_minutes >= 1 AND gap_pre_min_records >= 1 AND gap_post_min_records >= 1 AND shift_suppress_after_return_hours >= 1 AND abrupt_hours >= 1 AND abrupt_suppress_after_joint_hours >= 1 AND still_hours >= 1 AND still_min_calculated >= 1 AND temp_flat_hours >= 1 AND temp_flat_min_rates >= 1 AND temp_flow_hours >= 1 AND stabilize_max_hours >= 1 AND still_stable_first_hours >= 1 AND still_stable_every_hours >= 1"`

	TrendDeg7d float64 `gorm:"column:trend_deg_7d;type:numeric(6,4);not null"`

	// motion-v2 (§5.2). Los defaults dejan las versiones motion-v1 válidas.
	SendNaranja            string  `gorm:"column:send_naranja;type:text;not null;default:'OBSERVATION';check:chk_motion_threshold_versions_send,send_naranja IN ('ACTIVE', 'OBSERVATION') AND send_rojo IN ('ACTIVE', 'OBSERVATION')"`
	SendRojo               string  `gorm:"column:send_rojo;type:text;not null;default:'OBSERVATION'"`
	LongHoldHours          int     `gorm:"column:long_hold_hours;type:integer;not null;default:12"`
	LongMinCoverage        float64 `gorm:"column:long_min_coverage;type:numeric(3,2);not null;default:0.50;check:chk_motion_threshold_versions_fractions,long_min_coverage > 0 AND long_min_coverage <= 1 AND history_step_share > 0 AND history_step_share <= 1"`
	LongCloseHours         int     `gorm:"column:long_close_hours;type:integer;not null;default:72"`
	LongCloseNetDeg        float64 `gorm:"column:long_close_net_deg;type:numeric(6,4);not null;default:0.02"`
	ClosureCapLevel        string  `gorm:"column:closure_cap_level;type:text;not null;default:'AMARILLO';check:chk_motion_threshold_versions_closure_cap,closure_cap_level IN ('AMARILLO', 'NARANJA', 'ROJO')"`
	ClosureMaxDeg          float64 `gorm:"column:closure_max_deg;type:numeric(6,4);not null;default:0.20"`
	DailySummary           bool    `gorm:"column:daily_summary;not null;default:true"`
	HistoryStepDeg         float64 `gorm:"column:history_step_deg;type:numeric(6,4);not null;default:0.015"`
	HistoryStepWindowHours int     `gorm:"column:history_step_window_hours;type:integer;not null;default:6"`
	HistoryStepGroupHours  int     `gorm:"column:history_step_group_hours;type:integer;not null;default:8"`
	HistoryStepShare       float64 `gorm:"column:history_step_share;type:numeric(3,2);not null;default:0.70"`
	HistoryPreviousDays    int     `gorm:"column:history_previous_days;type:integer;not null;default:90"`

	// Rules es la tabla de lo normal de la versión (motion_threshold_rules).
	Rules []MotionThresholdRuleModel `gorm:"foreignKey:ThresholdVersionID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:RESTRICT"`

	ValidFrom time.Time  `gorm:"column:valid_from;type:timestamptz;not null;index:ix_motion_threshold_versions_effective,priority:2,sort:desc"`
	ValidTo   *time.Time `gorm:"column:valid_to;type:timestamptz;check:chk_motion_threshold_versions_validity,valid_to IS NULL OR valid_to > valid_from"`
	Reason    string     `gorm:"column:reason;type:text;not null"`
	CreatedBy string     `gorm:"column:created_by;type:text;not null"`
	CreatedAt time.Time  `gorm:"column:created_at;type:timestamptz;not null;default:now()"`
}

func (MotionThresholdVersionModel) TableName() string { return "motion_threshold_versions" }

// MotionThresholdRuleModel es una fila de motion_threshold_rules (§5.1): la
// tabla de lo normal de una versión. Las filas son de la versión: una versión
// nueva copia las de la vigente o trae las suyas.
type MotionThresholdRuleModel struct {
	ID                 int64    `gorm:"column:id;primaryKey;autoIncrement"`
	ThresholdVersionID int64    `gorm:"column:threshold_version_id;not null;uniqueIndex:ux_motion_threshold_rules_version_window_axis,priority:1"`
	WindowHours        int      `gorm:"column:window_hours;type:integer;not null;uniqueIndex:ux_motion_threshold_rules_version_window_axis,priority:2;check:chk_motion_threshold_rules_window,window_hours BETWEEN 1 AND 720"`
	Axis               string   `gorm:"column:axis;type:text;not null;uniqueIndex:ux_motion_threshold_rules_version_window_axis,priority:3;check:chk_motion_threshold_rules_axis,axis IN ('ANY', 'PITCH', 'ROLL')"`
	AmarilloDeg        float64  `gorm:"column:amarillo_deg;type:numeric(6,4);not null;check:chk_motion_threshold_rules_order,amarillo_deg > 0.01 AND (naranja_deg IS NULL OR naranja_deg > amarillo_deg) AND (rojo_deg IS NULL OR rojo_deg > COALESCE(naranja_deg, amarillo_deg))"`
	NaranjaDeg         *float64 `gorm:"column:naranja_deg;type:numeric(6,4)"`
	RojoDeg            *float64 `gorm:"column:rojo_deg;type:numeric(6,4)"`
}

func (MotionThresholdRuleModel) TableName() string { return "motion_threshold_rules" }

// MotionEvaluationModel es una fila de motion_evaluations (§6.2): una por punto
// y hora, también NO_CALCULABLE. Los grados y °C/h van en double precision
// para que el estado vuelva idéntico a la hora siguiente.
//
// Nombres: shift_sign es el del estado (§5.4); el de la hora es
// shift_sign_hour. gap_from / gap_to son los del estado; el hueco evaluado en
// la hora va en gap_eval_from / gap_eval_to / dp_gap / dr_gap.
type MotionEvaluationModel struct {
	ID               int64     `gorm:"column:id;primaryKey;autoIncrement"`
	MeasuringPointID int64     `gorm:"column:measuring_point_id;not null;uniqueIndex:ux_motion_evaluations_point_hour,priority:1"`
	EvaluatedAt      time.Time `gorm:"column:evaluated_at;type:timestamptz;not null;uniqueIndex:ux_motion_evaluations_point_hour,priority:2,sort:desc"`
	Status           string    `gorm:"column:status;type:text;not null;check:chk_motion_evaluations_status,status IN ('CALCULATED', 'NO_CALCULABLE')"`
	NRecords         int       `gorm:"column:n_records;type:integer;not null"`
	Coverage         float64   `gorm:"column:coverage;type:double precision;not null"`

	// Variables (§5.3).
	PitchNow       *float64 `gorm:"column:pitch_now;type:double precision"`
	RollNow        *float64 `gorm:"column:roll_now;type:double precision"`
	TempNow        *float64 `gorm:"column:temp_now;type:double precision"`
	DP1h           *float64 `gorm:"column:dp_1h;type:double precision"`
	DR1h           *float64 `gorm:"column:dr_1h;type:double precision"`
	BasePitch      *float64 `gorm:"column:base_pitch;type:double precision"`
	BaseRoll       *float64 `gorm:"column:base_roll;type:double precision"`
	DP6h           *float64 `gorm:"column:dp_6h;type:double precision"`
	DR6h           *float64 `gorm:"column:dr_6h;type:double precision"`
	ShiftRecords2h int      `gorm:"column:shift_records_2h;type:integer;not null;default:0"`
	ShiftSignHour  int      `gorm:"column:shift_sign_hour;type:integer;not null;default:0"`
	DerivaMax1h    *float64 `gorm:"column:deriva_max_1h;type:double precision"`
	Jump           bool     `gorm:"column:jump;not null;default:false"`
	Abrupt         bool     `gorm:"column:abrupt;not null;default:false"`
	DP24h          *float64 `gorm:"column:dp_24h;type:double precision"`
	DR24h          *float64 `gorm:"column:dr_24h;type:double precision"`
	SlowDriftFlag  bool     `gorm:"column:slow_drift_flag;not null;default:false"`
	TrendFlag      bool     `gorm:"column:trend_flag;not null;default:false"`
	TempRate       *float64 `gorm:"column:temp_rate;type:double precision"`
	TempRateMax6h  *float64 `gorm:"column:temp_rate_max_6h;type:double precision"`
	TempFlat       bool     `gorm:"column:temp_flat;not null;default:false"`
	TempFlow       bool     `gorm:"column:temp_flow;not null;default:false"`
	Still          bool     `gorm:"column:still;not null;default:false"`

	GapEvalFrom *time.Time `gorm:"column:gap_eval_from;type:timestamptz"`
	GapEvalTo   *time.Time `gorm:"column:gap_eval_to;type:timestamptz"`
	DPGap       *float64   `gorm:"column:dp_gap;type:double precision"`
	DRGap       *float64   `gorm:"column:dr_gap;type:double precision"`

	// Estado (§5.4).
	Displaced       bool       `gorm:"column:displaced;not null"`
	RefPitch        *float64   `gorm:"column:ref_pitch;type:double precision"`
	RefRoll         *float64   `gorm:"column:ref_roll;type:double precision"`
	ShiftSign       int        `gorm:"column:shift_sign;type:integer;not null;default:0;check:chk_motion_evaluations_shift_sign,shift_sign IN (-1, 0, 1) AND shift_sign_hour IN (-1, 0, 1)"`
	ShiftStartedAt  *time.Time `gorm:"column:shift_started_at;type:timestamptz"`
	ShiftLevelPitch *float64   `gorm:"column:shift_level_pitch;type:double precision"`
	ShiftLevelRoll  *float64   `gorm:"column:shift_level_roll;type:double precision"`
	GapFrom         *time.Time `gorm:"column:gap_from;type:timestamptz"`
	GapTo           *time.Time `gorm:"column:gap_to;type:timestamptz"`
	LastJointAt     *time.Time `gorm:"column:last_joint_at;type:timestamptz"`
	LastReturnAt    *time.Time `gorm:"column:last_return_at;type:timestamptz"`
	ReturnStartedAt *time.Time `gorm:"column:return_started_at;type:timestamptz"`
	TempState       *string    `gorm:"column:temp_state;type:text;check:chk_motion_evaluations_temp_state,temp_state IS NULL OR temp_state IN ('TEMP_VARIABLE', 'VERIFICANDO', 'TEMP_ESTABLE', 'RETORNO_TEMP_ESTABLE', 'SIN_ESTABILIZAR')"`
	TempSince       *time.Time `gorm:"column:temp_since;type:timestamptz"`
	TempStableFrom  *time.Time `gorm:"column:temp_stable_from;type:timestamptz"`
	TempStableTo    *time.Time `gorm:"column:temp_stable_to;type:timestamptz"`
	StillStableMsgs int        `gorm:"column:still_stable_msgs;type:integer;not null;default:0"`
	InEpisode       bool       `gorm:"column:in_episode;not null"`
	EpisodeOpenedAt *time.Time `gorm:"column:episode_opened_at;type:timestamptz"`
	AbruptActive    bool       `gorm:"column:abrupt_active;not null;default:false"`
	// PendingGaps es un arreglo JSON de PendingGapRow.
	PendingGaps string `gorm:"column:pending_gaps;type:jsonb;not null;default:'[]'"`

	// Resultado.
	MotionState   *string `gorm:"column:motion_state;type:text;check:chk_motion_evaluations_motion_state,motion_state IS NULL OR motion_state IN ('NORMAL', 'SALTO', 'DESPLAZADO', 'RETORNO', 'BRUSCO')"`
	EpisodeEvent  string  `gorm:"column:episode_event;type:text;not null;check:chk_motion_evaluations_episode_event,episode_event IN ('NONE', 'OPENED', 'CLEARED')"`
	ComputedLevel string  `gorm:"column:computed_level;type:text;not null;check:chk_motion_evaluations_computed_level,computed_level IN ('NORMAL', 'AMARILLO', 'NARANJA', 'ROJO')"`
	// Reasons es un arreglo JSON [{code, data}].
	Reasons string `gorm:"column:reasons;type:jsonb;not null;default:'[]'"`

	// motion-v2 (§7.1). NULL en las filas motion-v1 hasta el rebuild.
	// table_level_hour y rules_hit son NULL en las horas NO_CALCULABLE;
	// history solo en la hora OPENED.
	TableLevelHour *string    `gorm:"column:table_level_hour;type:text;check:chk_motion_evaluations_table_levels,(table_level_hour IS NULL OR table_level_hour IN ('NORMAL', 'AMARILLO', 'NARANJA', 'ROJO')) AND (table_level IS NULL OR table_level IN ('NORMAL', 'AMARILLO', 'NARANJA', 'ROJO'))"`
	TableLevel     *string    `gorm:"column:table_level;type:text"`
	EpisodeKind    *string    `gorm:"column:episode_kind;type:text;check:chk_motion_evaluations_episode_kind,episode_kind IS NULL OR episode_kind IN ('SALTO', 'HUECO', 'RAPIDO', 'ACUMULADO')"`
	RulesHit       *string    `gorm:"column:rules_hit;type:jsonb"`
	History        *string    `gorm:"column:history;type:jsonb"`
	RestAt         *time.Time `gorm:"column:rest_at;type:timestamptz"`
	LongHold       string     `gorm:"column:long_hold;type:jsonb;not null;default:'{}'"`
	LastPatternAt  *time.Time `gorm:"column:last_pattern_at;type:timestamptz"`
	LastFastAt     *time.Time `gorm:"column:last_fast_at;type:timestamptz"`
	EpStartAt      *time.Time `gorm:"column:ep_start_at;type:timestamptz"`
	EpRefPitch     *float64   `gorm:"column:ep_ref_pitch;type:double precision"`
	EpRefRoll      *float64   `gorm:"column:ep_ref_roll;type:double precision"`
	EpHasLong      bool       `gorm:"column:ep_has_long;not null;default:false"`

	// Trazabilidad.
	ThresholdVersionID int64                       `gorm:"column:threshold_version_id;not null"`
	ThresholdVersion   MotionThresholdVersionModel `gorm:"foreignKey:ThresholdVersionID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:RESTRICT"`
	AlgorithmVersion   string                      `gorm:"column:algorithm_version;type:text;not null"`
	Trigger            string                      `gorm:"column:trigger;type:text;not null;check:chk_motion_evaluations_trigger,trigger IN ('LIVE', 'CATCH_UP', 'BACKFILL')"`
	InputMaxTS         *time.Time                  `gorm:"column:input_max_ts;type:timestamptz"`
	CreatedAt          time.Time                   `gorm:"column:created_at;type:timestamptz;not null;default:now()"`
}

func (MotionEvaluationModel) TableName() string { return "motion_evaluations" }

// PendingGapRow es un elemento de motion_evaluations.pending_gaps.
type PendingGapRow struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// MotionReasonRow es un elemento de motion_evaluations.reasons.
type MotionReasonRow struct {
	Code string         `json:"code"`
	Data map[string]any `json:"data"`
}

// RawMeasurementReadModel es la vista de SOLO LECTURA del Rule Engine sobre
// raw_measurements (la escribe Ingestion). NO se migra.
type RawMeasurementReadModel struct {
	MeasuringPointID int64     `gorm:"column:measuring_point_id"`
	MetricID         int64     `gorm:"column:metric_id"`
	Timestamp        time.Time `gorm:"column:timestamp"`
	Value            float64   `gorm:"column:value"`
}

func (RawMeasurementReadModel) TableName() string { return "raw_measurements" }
